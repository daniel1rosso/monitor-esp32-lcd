export type ServiceState = 'operational' | 'degraded' | 'outage' | 'maintenance' | 'unknown'
export type SemanticPriority = 'critical' | 'warning' | 'info' | 'success'
export type Trend = 'up' | 'down' | 'flat' | 'none'

export interface NotificationStyle {
  color: string
  icon: string
  led_rgb: string
  sound: 'none' | 'future_default'
}

interface ScreenBase<T extends string, P> {
  id: string
  type: T
  priority: number
  duration_ms: number
  title: string
  payload: P
}

export type DeviceScreen =
  | ScreenBase<'overview', { status: ServiceState; products: number; services: number; open_alerts: number; subtitle?: string }>
  | ScreenBase<'product_status', { product_key: string; name: string; status: ServiceState; services_up: number; services_total: number; detail?: string }>
  | ScreenBase<'service_status', { name: string; status: ServiceState; latency_ms?: number | null; last_change_at?: string | null; detail?: string }>
  | ScreenBase<'alert', { alert_id: string; severity: SemanticPriority; message: string; source?: string; style: NotificationStyle }>
  | ScreenBase<'deployment', { repository: string; branch: string; workflow: string; status: 'queued' | 'in_progress' | 'success' | 'failure' | 'cancelled'; commit_short: string; author: string; occurred_at?: string }>
  | ScreenBase<'market', { symbol: string; display_value: string; bid?: string | null; ask?: string | null; change_percent: number; trend: Exclude<Trend, 'none'>; stale?: boolean }>
  | ScreenBase<'weather', { location: string; temperature_c: number; apparent_temperature_c?: number; condition_icon: string; daily_min_c: number; daily_max_c: number; precipitation_probability_percent?: number; stale?: boolean }>
  | ScreenBase<'metric_list', { items: Array<{ label: string; value: string; trend?: Trend; color?: string }> }>
  | ScreenBase<'message', { message: string; style: NotificationStyle }>
