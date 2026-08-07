import { render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'
import { DeviceScreen } from './DeviceScreen'
import { deviceScreenFixtures } from './fixtures'

describe('DeviceScreen', () => {
  test.each(deviceScreenFixtures)('renders $type fixture', (fixture) => {
    render(<DeviceScreen screen={fixture} />)
    expect(screen.getByTestId(`device-screen-${fixture.type}`)).toBeTruthy()
  })
})
