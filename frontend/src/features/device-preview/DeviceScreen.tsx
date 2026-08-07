import { Icon } from '../../components/icons/Icon'
import type { CSSProperties } from 'react'
import type { DeviceScreen as DeviceScreenValue, ServiceState, Trend } from './types'

const stateLabels: Record<ServiceState, string> = { operational: 'Operativo', degraded: 'Degradado', outage: 'Caído', maintenance: 'Mantenimiento', unknown: 'Sin datos' }
const screenIcons = { overview: 'activity', product_status: 'box', service_status: 'server', alert: 'alert', deployment: 'code', market: 'dollar', weather: 'cloud', metric_list: 'activity', message: 'message' } as const

function TrendIcon({ trend }: { trend?: Trend }) {
  if (!trend || trend === 'none' || trend === 'flat') return <span className="device-flat">—</span>
  return <Icon name={trend === 'up' ? 'trend-up' : 'trend-down'} size={12} />
}

function StateBadge({ state }: { state: ServiceState }) { return <span className={`device-state device-state-${state}`}>{stateLabels[state]}</span> }

function ScreenBody({ screen }: { screen: DeviceScreenValue }) {
  switch (screen.type) {
    case 'overview': return <><div className="device-hero-state"><span className={`device-orb device-orb-${screen.payload.status}`} /><strong>{stateLabels[screen.payload.status]}</strong></div><div className="device-stat-grid"><div><strong>{screen.payload.products}</strong><span>Productos</span></div><div><strong>{screen.payload.services}</strong><span>Servicios</span></div><div><strong>{screen.payload.open_alerts}</strong><span>Alertas</span></div></div><p className="device-detail">{screen.payload.subtitle}</p></>
    case 'product_status': { const ratio = screen.payload.services_total ? (screen.payload.services_up / screen.payload.services_total) * 100 : 0; return <><StateBadge state={screen.payload.status} /><div className="device-product-name">{screen.payload.name}</div><div className="device-ratio"><strong>{screen.payload.services_up}</strong><span>/ {screen.payload.services_total} servicios</span></div><div className="device-progress"><span style={{ width: `${ratio}%` }} /></div><p className="device-detail">{screen.payload.detail}</p></> }
    case 'service_status': return <><StateBadge state={screen.payload.status} /><div className="device-product-name">{screen.payload.name}</div><div className="device-latency"><strong>{screen.payload.latency_ms ?? '—'}</strong><span> ms</span></div><p className="device-detail">{screen.payload.detail}</p><small>Último cambio · hace 42 min</small></>
    case 'alert': return <div className="device-alert" style={{ '--alert-color': screen.payload.style.color } as CSSProperties}><div className="device-alert-icon"><Icon name="alert" size={30} /></div><strong>{screen.title}</strong><p>{screen.payload.message}</p><small>{screen.payload.source}</small></div>
    case 'deployment': return <><div className={`device-deploy-status device-deploy-${screen.payload.status}`}><Icon name={screen.payload.status === 'success' ? 'check' : 'code'} size={18} /><span>{screen.payload.status.replace('_', ' ')}</span></div><strong className="device-repo">{screen.payload.repository}</strong><div className="device-code-row"><code>{screen.payload.branch}</code><code>{screen.payload.commit_short}</code></div><p className="device-detail">{screen.payload.workflow}</p><small>por {screen.payload.author}</small></>
    case 'market': return <><div className="device-symbol">{screen.payload.symbol}</div><div className="device-price">{screen.payload.display_value}</div><div className={`device-change device-change-${screen.payload.trend}`}><TrendIcon trend={screen.payload.trend} />{screen.payload.change_percent > 0 ? '+' : ''}{screen.payload.change_percent.toFixed(2)}%</div><div className="device-bidask"><span>Compra<strong>{screen.payload.bid ?? '—'}</strong></span><span>Venta<strong>{screen.payload.ask ?? '—'}</strong></span></div>{screen.payload.stale && <span className="device-stale">Datos demorados</span>}</>
    case 'weather': return <><div className="device-weather"><Icon name="cloud" size={42} /><div><strong>{screen.payload.temperature_c.toFixed(1)}°</strong><span>Sensación {screen.payload.apparent_temperature_c?.toFixed(1) ?? '—'}°</span></div></div><div className="device-location">{screen.payload.location}</div><div className="device-stat-grid device-stat-grid-two"><div><strong>{screen.payload.daily_min_c.toFixed(0)}°</strong><span>Mínima</span></div><div><strong>{screen.payload.daily_max_c.toFixed(0)}°</strong><span>Máxima</span></div></div><p className="device-detail">Lluvia {screen.payload.precipitation_probability_percent ?? 0}%</p></>
    case 'metric_list': return <div className="device-metrics">{screen.payload.items.map((item) => <div key={item.label}><span><i style={{ background: item.color }} />{item.label}</span><strong>{item.value}<TrendIcon trend={item.trend} /></strong></div>)}</div>
    case 'message': return <div className="device-message" style={{ '--message-color': screen.payload.style.color } as CSSProperties}><span><Icon name={screen.payload.style.icon === 'wrench' ? 'wrench' : 'info'} size={28} /></span><p>{screen.payload.message}</p></div>
  }
}

export function DeviceScreen({ screen, index = 0, total = 1, connected = true }: { screen: DeviceScreenValue; index?: number; total?: number; connected?: boolean }) {
  return <article className={`device-screen device-screen-${screen.type}`} data-testid={`device-screen-${screen.type}`}>
    <header className="device-header"><span className="device-header-icon"><Icon name={screenIcons[screen.type]} size={15} /></span><h3>{screen.title}</h3><span className={`device-connection ${connected ? 'is-online' : ''}`} /></header>
    <section className="device-body"><ScreenBody screen={screen} /></section>
    <footer className="device-footer"><span>{connected ? 'Actualizado ahora' : 'Sin conexión'}</span><span>{index + 1}/{total}</span><div className="device-rotation-progress"><span style={{ width: '62%' }} /></div></footer>
  </article>
}
