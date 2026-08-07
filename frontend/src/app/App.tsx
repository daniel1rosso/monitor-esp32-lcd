import { useQuery } from '@tanstack/react-query'
import { AnimatePresence, motion } from 'framer-motion'
import { useEffect, useMemo, useState } from 'react'

import { api, type Alert, type Device, type Product, type Service } from '../api/client'
import { Icon } from '../components/icons/Icon'
import { DeviceShowcase } from '../features/device-preview/DeviceShowcase'

const pages = [
  ['dashboard', 'Centro de monitoreo', 'activity'], ['products', 'Productos', 'box'],
  ['alerts', 'Alertas', 'alert'], ['deployments', 'Deployments', 'code'],
  ['markets', 'Mercados', 'dollar'], ['weather', 'Clima', 'cloud'],
  ['devices', 'Dispositivos', 'server'], ['preview', 'Simulador LCD', 'message'],
  ['settings', 'Configuración', 'wrench'],
] as const
type Page = typeof pages[number][0]

const fallbackProducts: Product[] = [
  { id:'potrerito', key:'potrerito', name:'Potrerito', description:'Operaciones internas', enabled:true, health:'operational' },
  { id:'freshcode', key:'freshcode', name:'FreshCode', description:'Servicios comerciales', enabled:true, health:'operational' },
  { id:'facturas', key:'facturas', name:'Facturas-FreshCode', description:'Facturación', enabled:true, health:'degraded' },
]
const fallbackServices: Service[] = [
  {id:'api', product_id:'potrerito', key:'api', name:'API principal', kind:'http', enabled:true, status:'operational', latency_ms:84},
  {id:'billing', product_id:'facturas', key:'billing', name:'Facturación', kind:'http', enabled:true, status:'degraded', latency_ms:312},
  {id:'web', product_id:'freshcode', key:'web', name:'Sitio web', kind:'http', enabled:true, status:'operational', latency_ms:47},
]

function currentPage(): Page {
  const value = location.hash.slice(1) as Page
  return pages.some(([key]) => key === value) ? value : 'dashboard'
}

export function App() {
  const [page, setPage] = useState<Page>(currentPage)
  const [sidebar, setSidebar] = useState(false)
  useEffect(() => { const onHash=()=>setPage(currentPage()); addEventListener('hashchange',onHash); return()=>removeEventListener('hashchange',onHash) }, [])
  const health = useQuery({ queryKey:['health'], queryFn:()=>api<{status:string}>('/health'), refetchInterval:30_000 })
  const productsQuery = useQuery({ queryKey:['products'], queryFn:()=>api<{items:Product[]}>('/products'), retry:false })
  const servicesQuery = useQuery({ queryKey:['services'], queryFn:()=>api<{items:Service[]}>('/services'), retry:false })
  const alertsQuery = useQuery({ queryKey:['alerts'], queryFn:()=>api<{items:Alert[]}>('/alerts?state=open'), retry:false })
  const devicesQuery = useQuery({ queryKey:['devices'], queryFn:()=>api<{items:Device[]}>('/devices'), retry:false })
  const products=productsQuery.data?.items ?? fallbackProducts, services=servicesQuery.data?.items ?? fallbackServices, alerts=alertsQuery.data?.items ?? [], devices=devicesQuery.data?.items ?? []
  const connected = health.data?.status === 'ok'
  const title = pages.find(([key])=>key===page)?.[1] ?? ''
  const select=(key:Page)=>{ location.hash=key; setSidebar(false) }
  return <div className="admin-shell">
    <div className="ambient ambient-left"/><div className="ambient ambient-right"/>
    <aside className={`sidebar ${sidebar?'is-open':''}`}>
      <div className="brand"><span className="brand-mark"><Icon name="activity"/></span><div><strong>Desk Monitor</strong><small>Platform</small></div></div>
      <nav aria-label="Navegación principal">{pages.map(([key,label,icon])=><button key={key} className={page===key?'active':''} onClick={()=>select(key)}><Icon name={icon}/><span>{label}</span>{key==='alerts'&&alerts.length>0?<b>{alerts.length}</b>:null}</button>)}</nav>
      <div className="sidebar-status"><span className={connected?'live-dot':'live-dot pending'}/><div><strong>{connected?'Backend conectado':'Modo demostración'}</strong><small>{connected?'API operacional':'Esperando autenticación/API'}</small></div></div>
    </aside>
    <main className="workspace">
      <header className="topbar"><button className="mobile-menu" onClick={()=>setSidebar(!sidebar)}>☰</button><div><span className="eyebrow">OPERATIONS / {page.toUpperCase()}</span><h1>{title}</h1></div><div className="top-actions"><span className="sync-state"><span className={connected?'live-dot':'live-dot pending'}/>{connected?'En vivo':'Demo'}</span><div className="avatar">DR</div></div></header>
      <AnimatePresence mode="wait"><motion.div key={page} className="page" initial={{opacity:0,y:8}} animate={{opacity:1,y:0}} exit={{opacity:0,y:-5}} transition={{duration:.2}}>
        <PageContent page={page} products={products} services={services} alerts={alerts} devices={devices}/>
      </motion.div></AnimatePresence>
    </main>
  </div>
}

function PageContent({page,products,services,alerts,devices}:{page:Page;products:Product[];services:Service[];alerts:Alert[];devices:Device[]}) {
  if(page==='preview') return <DeviceShowcase/>
  if(page==='dashboard') return <Dashboard products={products} services={services} alerts={alerts} devices={devices}/>
  if(page==='products') return <ResourcePage eyebrow="Portfolio" title="Productos configurados" action="Nuevo producto"><DataTable headers={['Producto','Identificador','Estado','Descripción']} rows={products.map(p=>[p.name,p.key,<State key={p.id} value={p.health}/>,p.description||'Sin descripción'])}/></ResourcePage>
  if(page==='alerts') return <ResourcePage eyebrow="Incidentes" title="Alertas activas" action="Crear alerta"><DataTable headers={['Prioridad','Título','Estado','Mensaje']} rows={alerts.length?alerts.map(a=>[<State key={a.id} value={a.priority}/>,a.title,a.state,a.message]):[[<State key="none" value="success"/>,'Sin alertas activas','—','La plataforma no reporta incidentes']]}/></ResourcePage>
  if(page==='devices') return <ResourcePage eyebrow="Fleet" title="Dispositivos registrados" action="Provisionar dispositivo"><DataTable headers={['Dispositivo','ID','Estado','Firmware']} rows={devices.length?devices.map(d=>[d.name,d.device_id,<State key={d.id} value={d.state}/>,d.firmware_version??'Sin reporte']):[['Sin dispositivos','—',<State key="off" value="offline"/>,'Provisioná el primer ESP32']]}/></ResourcePage>
  if(page==='deployments') return <EmptyInsight icon="code" title="Deployments" text="Los webhooks de GitHub Actions aparecerán acá con repositorio, branch, workflow, commit y autor."/>
  if(page==='markets') return <MarketPage/>
  if(page==='weather') return <WeatherPage/>
  return <SettingsPage/>
}

function Dashboard({products,services,alerts,devices}:{products:Product[];services:Service[];alerts:Alert[];devices:Device[]}) {
  const operational=services.filter(s=>s.status==='operational').length
  const activity=useMemo(()=>[
    ['API principal operacional','Potrerito · hace 2 min','success'],['Deploy completado','freshcode/main · hace 18 min','info'],['Latencia elevada','Facturación · hace 24 min','warning']],[])
  return <><section className="hero-panel glass"><div><span className="eyebrow">ESTADO GENERAL</span><h2>Todo bajo control.</h2><p>Una vista única de productos, servicios y dispositivos en tiempo real.</p></div><div className="hero-orbit"><span/><strong>{alerts.length}</strong><small>alertas activas</small></div></section>
    <section className="kpi-grid"><Metric icon="box" label="Productos" value={products.length} detail="configurados"/><Metric icon="server" label="Servicios" value={`${operational}/${services.length}`} detail="operacionales"/><Metric icon="alert" label="Alertas" value={alerts.length} detail="requieren atención"/><Metric icon="activity" label="Dispositivos" value={devices.filter(d=>d.state==='online').length} detail={`${devices.length} registrados`}/></section>
    <section className="dashboard-grid"><div className="panel"><PanelTitle title="Salud de productos" subtitle="Estado agregado por servicio"/><div className="health-list">{products.map((p,i)=><div key={p.id}><span className={`health-icon tone-${p.health}`}><Icon name="box"/></span><div><strong>{p.name}</strong><small>{services.filter(s=>s.product_id===p.id).length} servicios</small></div><State value={p.health}/><i style={{width:`${92-i*9}%`}}/></div>)}</div></div><div className="panel"><PanelTitle title="Actividad reciente" subtitle="Eventos operacionales"/><div className="timeline">{activity.map(([name,detail,state])=><div key={name}><span className={`live-dot tone-${state}`}/><div><strong>{name}</strong><small>{detail}</small></div></div>)}</div></div></section>
  </>
}

function ResourcePage({eyebrow,title,action,children}:{eyebrow:string;title:string;action:string;children:React.ReactNode}) { return <><div className="section-heading"><div><span className="eyebrow">{eyebrow}</span><h2>{title}</h2></div><button className="primary-action">＋ {action}</button></div><div className="panel table-panel">{children}</div></> }
function DataTable({headers,rows}:{headers:string[];rows:React.ReactNode[][]}) { return <div className="data-table"><div className="table-row table-head">{headers.map(x=><span key={x}>{x}</span>)}</div>{rows.map((row,i)=><div className="table-row" key={i}>{row.map((cell,j)=><span key={j}>{cell}</span>)}</div>)}</div> }
function State({value}:{value:string}) { const label:{[k:string]:string}={operational:'Operacional',degraded:'Degradado',outage:'Caído',unknown:'Sin datos',online:'Online',offline:'Offline',critical:'Crítica',warning:'Warning',info:'Info',success:'OK'}; return <span className={`state-chip tone-${value}`}>{label[value]??value}</span> }
function Metric({icon,label,value,detail}:{icon:'box'|'server'|'alert'|'activity';label:string;value:string|number;detail:string}) { return <div className="metric-card"><span><Icon name={icon}/></span><small>{label}</small><strong>{value}</strong><p>{detail}</p></div> }
function PanelTitle({title,subtitle}:{title:string;subtitle:string}) { return <div className="panel-title"><div><h3>{title}</h3><p>{subtitle}</p></div><button>•••</button></div> }
function EmptyInsight({icon,title,text}:{icon:'code';title:string;text:string}) { return <div className="empty-insight glass"><span><Icon name={icon} size={28}/></span><h2>{title}</h2><p>{text}</p><b>Listo para recibir eventos</b></div> }
function MarketPage(){return <><div className="section-heading"><div><span className="eyebrow">COTIZACIÓN YA</span><h2>Mercados</h2></div><span className="sync-state"><span className="live-dot pending"/>Esperando colector</span></div><div className="quote-grid">{[['Dólar Blue','$ 1.285,00','+0,8%'],['Bitcoin','US$ 116.420','+2,4%'],['XRP','US$ 3,12','−0,6%']].map(([a,b,c],i)=><div className="quote-card" key={a}><small>{a}</small><strong>{b}</strong><span className={i===2?'down':''}>{c}</span><div className="sparkline">▁▂▃▂▄▅▄▆▇</div></div>)}</div></>}
function WeatherPage(){return <div className="weather-hero glass"><span><Icon name="cloud" size={52}/></span><div><span className="eyebrow">RÍO CUARTO · CÓRDOBA</span><strong>22°</strong><p>Parcialmente nublado · sensación 21°</p></div><dl><div><dt>Mínima</dt><dd>14°</dd></div><div><dt>Máxima</dt><dd>25°</dd></div><div><dt>Lluvia</dt><dd>18%</dd></div></dl></div>}
function SettingsPage(){return <div className="settings-grid"><div className="panel"><PanelTitle title="Plataforma" subtitle="Configuración efectiva desde YAML"/><label>Zona horaria<input value="America/Argentina/Cordoba" readOnly/></label><label>Perfil predeterminado<input value="operations" readOnly/></label><label>Intervalo del dashboard<input value="5 minutos" readOnly/></label></div><div className="panel architecture-card"><span><Icon name="check"/></span><h3>Arquitectura lista</h3><p>La fuente de verdad permanece versionada en YAML. Los cambios administrativos serán persistidos con revisión y auditoría.</p><code>contracts/config/platform.example.yaml</code></div></div>}
