import type { DeviceScreen } from './types'

const base = { priority: 40, duration_ms: 8000 }

export const deviceScreenFixtures: DeviceScreen[] = [
  { ...base, id: 'overview-main', type: 'overview', title: 'Estado general', payload: { status: 'operational', products: 3, services: 8, open_alerts: 0, subtitle: 'Todos los sistemas operativos' } },
  { ...base, id: 'product-main', type: 'product_status', title: 'FreshCode', payload: { product_key: 'freshcode', name: 'FreshCode', status: 'degraded', services_up: 4, services_total: 5, detail: 'Un servicio presenta latencia' } },
  { ...base, id: 'service-main', type: 'service_status', title: 'API principal', payload: { name: 'Potrerito API', status: 'operational', latency_ms: 84, last_change_at: '2026-08-06T14:20:00Z', detail: 'Respondiendo normalmente' } },
  { ...base, id: 'alert-main', type: 'alert', priority: 100, duration_ms: 15000, title: 'Servicio caído', payload: { alert_id: 'alert-01', severity: 'critical', message: 'Potrerito API no responde', source: 'Uptime Kuma', style: { color: '#FF3B30', icon: 'alert-octagon', led_rgb: '#FF0000', sound: 'future_default' } } },
  { ...base, id: 'deployment-main', type: 'deployment', title: 'Último deploy', payload: { repository: 'freshcode/api', branch: 'main', workflow: 'Production', status: 'success', commit_short: 'a84e12c', author: 'Daniel', occurred_at: '2026-08-06T14:50:00Z' } },
  { ...base, id: 'market-main', type: 'market', title: 'Bitcoin', payload: { symbol: 'BTC/USDT', display_value: '84.500,12', bid: '84.498,50', ask: '84.501,30', change_percent: 2.1, trend: 'up', stale: false } },
  { ...base, id: 'weather-main', type: 'weather', title: 'Clima', payload: { location: 'Río Cuarto', temperature_c: 18.4, apparent_temperature_c: 17.2, condition_icon: 'partly-cloudy', daily_min_c: 8.1, daily_max_c: 22.5, precipitation_probability_percent: 20, stale: false } },
  { ...base, id: 'metrics-main', type: 'metric_list', title: 'Métricas', payload: { items: [{ label: 'Usuarios activos', value: '1.248', trend: 'up', color: '#30D158' }, { label: 'Facturas hoy', value: '386', trend: 'up', color: '#0A84FF' }, { label: 'Errores', value: '3', trend: 'down', color: '#FFCC00' }] } },
  { ...base, id: 'message-main', type: 'message', title: 'Mantenimiento', payload: { message: 'Ventana programada hoy a las 23:00', style: { color: '#BF5AF2', icon: 'wrench', led_rgb: '#BF5AF2', sound: 'none' } } },
]
