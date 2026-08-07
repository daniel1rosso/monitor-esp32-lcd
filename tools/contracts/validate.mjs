import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import SwaggerParser from '@apidevtools/swagger-parser'
import { DiagnosticSeverity, fromFile, Parser } from '@asyncapi/parser'
import Ajv2020 from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'
import YAML from 'yaml'

const toolDirectory = path.dirname(fileURLToPath(import.meta.url))
const root = path.resolve(toolDirectory, '../..')
const schemasDirectory = path.join(root, 'contracts/json-schema')

const loadJSON = (file) => JSON.parse(fs.readFileSync(file, 'utf8'))

function createValidator() {
  const validator = new Ajv2020({ allErrors: true, strict: true })
  addFormats(validator)
  return validator
}

function assertValid(validator, schema, data, label) {
  if (!validator.validate(schema, data)) {
    throw new Error(`${label}: ${validator.errorsText(validator.errors)}`)
  }
}

function validateJSONContracts() {
  const validator = createValidator()
  const schemaNames = [
    'dashboard.schema.json',
    'device-config.schema.json',
    'mqtt-envelope.schema.json',
  ]

  for (const name of schemaNames) {
    validator.addSchema(loadJSON(path.join(schemasDirectory, name)))
  }

  const fixtures = [
    ['https://desk-monitor.local/contracts/v1/dashboard.schema.json', 'dashboard.json'],
    ['https://desk-monitor.local/contracts/v1/device-config.schema.json', 'device-config.json'],
    ['https://desk-monitor.local/contracts/v1/mqtt-envelope.schema.json', 'mqtt-alert.json'],
  ]

  for (const [schema, fixture] of fixtures) {
    assertValid(
      validator,
      schema,
      loadJSON(path.join(schemasDirectory, 'fixtures', fixture)),
      fixture,
    )
  }

  const invalidAlert = loadJSON(path.join(schemasDirectory, 'fixtures/mqtt-alert.json'))
  invalidAlert.data = {
    kind: 'telemetry',
    uptime_seconds: 1,
    free_heap_bytes: 1,
    wifi_rssi_dbm: -50,
  }
  if (validator.validate('https://desk-monitor.local/contracts/v1/mqtt-envelope.schema.json', invalidAlert)) {
    throw new Error('MQTT type/data mismatch was accepted')
  }

  const invalidDashboard = loadJSON(path.join(schemasDirectory, 'fixtures/dashboard.json'))
  invalidDashboard.screens[0].unexpected = true
  if (validator.validate('https://desk-monitor.local/contracts/v1/dashboard.schema.json', invalidDashboard)) {
    throw new Error('Unknown screen property was accepted')
  }
}

function validateConfiguration() {
  const validator = createValidator()
  const schema = loadJSON(path.join(root, 'contracts/config/platform-config.schema.json'))
  const config = YAML.parse(
    fs.readFileSync(path.join(root, 'contracts/config/platform.example.yaml'), 'utf8'),
  )
  assertValid(validator, schema, config, 'platform.example.yaml')

  config.unexpected = true
  if (validator.validate(schema, config)) {
    throw new Error('Unknown root configuration property was accepted')
  }
}

function walk(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    if (entry.name === '.git' || entry.name === 'node_modules') return []
    const target = path.join(directory, entry.name)
    return entry.isDirectory() ? walk(target) : [target]
  })
}

function validateMarkdownLinks() {
  const missing = []
  const expression = /\[[^\]]+]\(([^)]+)\)/g

  for (const file of walk(root).filter((candidate) => candidate.endsWith('.md'))) {
    const content = fs.readFileSync(file, 'utf8')
    for (const match of content.matchAll(expression)) {
      const target = match[1]
      if (target.startsWith('http://') || target.startsWith('https://') || target.startsWith('#')) {
        continue
      }
      const localTarget = path.resolve(path.dirname(file), target.split('#', 1)[0])
      if (!fs.existsSync(localTarget)) {
        missing.push(`${path.relative(root, file)} -> ${target}`)
      }
    }
  }

  if (missing.length > 0) {
    throw new Error(`Broken Markdown links:\n${missing.join('\n')}`)
  }
}

async function validateAPIs() {
  const openapi = await SwaggerParser.validate(path.join(root, 'contracts/openapi/openapi.yaml'))
  const operations = Object.values(openapi.paths).flatMap((pathItem) =>
    Object.values(pathItem).filter((operation) => operation?.operationId),
  )
  const operationIDs = operations.map((operation) => operation.operationId)
  if (new Set(operationIDs).size !== operationIDs.length) {
    throw new Error('OpenAPI operationId values must be unique')
  }

  const asyncapi = await fromFile(
    new Parser(),
    path.join(root, 'contracts/mqtt/asyncapi.yaml'),
  ).parse()
  const diagnostics = asyncapi.diagnostics.filter(
    (diagnostic) => diagnostic.severity <= DiagnosticSeverity.Warning,
  )
  if (diagnostics.length > 0) {
    throw new Error(diagnostics.map((diagnostic) => diagnostic.message).join('\n'))
  }

  return { paths: Object.keys(openapi.paths).length }
}

validateJSONContracts()
validateConfiguration()
validateMarkdownLinks()
const summary = await validateAPIs()
console.log(`Contracts valid: ${summary.paths} OpenAPI paths, JSON Schema, AsyncAPI and Markdown`)

