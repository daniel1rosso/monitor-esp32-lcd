import { DeviceScreen } from './DeviceScreen'
import { deviceScreenFixtures } from './fixtures'

export function DeviceShowcase() {
  return <section className="device-showcase"><div className="device-showcase-heading"><div><span className="eyebrow">ESP32 · 172×320</span><h2>Catálogo contractual</h2><p>Nueve layouts alimentados por el mismo modelo que consumirá el firmware.</p></div><span className="device-showcase-count">9 pantallas</span></div><div className="device-showcase-grid">{deviceScreenFixtures.map((screen,index)=><div className="device-preview-card" key={screen.id}><DeviceScreen screen={screen} index={index} total={deviceScreenFixtures.length}/><code>{screen.type}</code></div>)}</div></section>
}
