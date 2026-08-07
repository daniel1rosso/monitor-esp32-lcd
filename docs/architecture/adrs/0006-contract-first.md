# ADR 0006: Contratos públicos como fuente de verdad

- Estado: Accepted
- Fecha: 2026-08-06

## Decisión

OpenAPI 3.1 define REST y genera el cliente TypeScript. JSON Schema 2020-12 define
dashboard/config y envelopes MQTT. AsyncAPI documenta topics, QoS y seguridad. Los
fixtures compartidos prueban backend, frontend y firmware.

Los contratos llevan versión mayor. Campos nuevos opcionales son compatibles;
eliminar, renombrar o cambiar semántica requiere nueva versión. El firmware omite
tipos de pantalla desconocidos y conserva las pantallas válidas.

## Consecuencias

Comentarios Go no son la fuente de Swagger y los DTO no evolucionan aisladamente.
CI bloquea referencias rotas, ejemplos inválidos y cambios incompatibles.

