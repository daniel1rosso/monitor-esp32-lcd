import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import '../i18n'
import { App } from './App'

test('renders the product scaffold', () => {
	localStorage.setItem('desk_access_token','test-token')
	vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')))
  render(
    <QueryClientProvider client={new QueryClient()}>
      <App />
    </QueryClientProvider>,
  )

	expect(screen.getAllByText('Centro de monitoreo').length).toBeGreaterThan(0)
	expect(screen.getByText('Todo bajo control.')).toBeTruthy()
	localStorage.clear()
})
