import { useQuery } from '@tanstack/react-query'
import { AnimatePresence, motion } from 'framer-motion'
import { type FormEvent, useEffect, useState } from 'react'

import { api, APIError, login, logout, type Alert, type Deployment, type Device, type MarketQuote, type Product, type Service, type Weather } from '../api/client'
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

function currentPage(): Page {
  const value = location.hash.slice(1) as Page
  return pages.some(([key]) => key === value) ? value : 'dashboard'
}

export function App() {
  const [page, setPage] = useState<Page>(currentPage)
  const [sidebar, setSidebar] = useState(false)
  const [authenticated,setAuthenticated]=useState(()=>Boolean(localStorage.getItem('desk_access_token')))
  useEffect(() => { const onHash=()=>setPage(currentPage()); addEventListener('hashchange',onHash); return()=>removeEventListener('hashchange',onHash) }, [])
  const health = useQuery({ queryKey:['health'], queryFn:()=>api<{status:string}>('/health'), refetchInterval:30_000 })
  const productsQuery = useQuery({ queryKey:['products'], queryFn:()=>api<{items:Product[]}>('/products'), retry:false,enabled:authenticated,refetchInterval:30_000 })
  const servicesQuery = useQuery({ queryKey:['services'], queryFn:()=>api<{items:Service[]}>('/services'), retry:false,enabled:authenticated,refetchInterval:30_000 })
  const alertsQuery = useQuery({ queryKey:['alerts'], queryFn:()=>api<{items:Alert[]}>('/alerts?state=open'), retry:false,enabled:authenticated,refetchInterval:15_000 })
  const devicesQuery = useQuery({ queryKey:['devices'], queryFn:()=>api<{items:Device[]}>('/devices'), retry:false,enabled:authenticated,refetchInterval:30_000 })
  const deploymentsQuery = useQuery({queryKey:['deployments'],queryFn:()=>api<{items:Deployment[]}>('/deployments?limit=50'),retry:false,enabled:authenticated,refetchInterval:30_000})
  const marketsQuery = useQuery({queryKey:['markets'],queryFn:()=>api<{items:MarketQuote[]}>('/markets/quotes?limit=100'),retry:false,enabled:authenticated,refetchInterval:60_000})
  const weatherQuery = useQuery({queryKey:['weather'],queryFn:()=>api<Weather>('/weather?location=rio-cuarto'),retry:false,enabled:authenticated,refetchInterval:60_000})
  const unauthorized=[productsQuery,servicesQuery,alertsQuery,devicesQuery,deploymentsQuery,marketsQuery,weatherQuery].some(query=>query.error instanceof APIError&&query.error.status===401)
  useEffect(()=>{if(unauthorized){localStorage.removeItem('desk_access_token');setAuthenticated(false)}},[unauthorized])
  if(!authenticated)return <Login onAuthenticated={()=>setAuthenticated(true)} backendOnline={health.data?.status==='ok'}/>
  const products=productsQuery.data?.items ?? [], services=servicesQuery.data?.items ?? [], alerts=alertsQuery.data?.items ?? [], devices=devicesQuery.data?.items ?? []
  const deployments=deploymentsQuery.data?.items??[], quotes=marketsQuery.data?.items??[]
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
      <header className="topbar"><button className="mobile-menu" onClick={()=>setSidebar(!sidebar)}>☰</button><div><span className="eyebrow">OPERATIONS / {page.toUpperCase()}</span><h1>{title}</h1></div><div className="top-actions"><span className="sync-state"><span className={connected?'live-dot':'live-dot pending'}/>{connected?'En vivo':'Sin conexión'}</span><button className="avatar logout" title="Cerrar sesión" onClick={async()=>{try{await logout()}finally{localStorage.removeItem('desk_access_token');setAuthenticated(false)}}}>DR</button></div></header>
      <AnimatePresence mode="wait"><motion.div key={page} className="page" initial={{opacity:0,y:8}} animate={{opacity:1,y:0}} exit={{opacity:0,y:-5}} transition={{duration:.2}}>
        <PageContent page={page} products={products} services={services} alerts={alerts} devices={devices} deployments={deployments} quotes={quotes} weather={weatherQuery.data}/>
      </motion.div></AnimatePresence>
    </main>
  </div>
}

function PageContent({page,products,services,alerts,devices,deployments,quotes,weather}:{page:Page;products:Product[];services:Service[];alerts:Alert[];devices:Device[];deployments:Deployment[];quotes:MarketQuote[];weather?:Weather}) {
  if(page==='preview') return <DeviceShowcase/>
  if(page==='dashboard') return <Dashboard products={products} services={services} alerts={alerts} devices={devices} deployments={deployments}/>
  if(page==='products') return <ResourcePage eyebrow="Portfolio" title="Productos configurados" action="Nuevo producto"><DataTable headers={['Producto','Identificador','Estado','Descripción']} rows={products.map(p=>[p.name,p.key,<State key={p.id} value={p.health}/>,p.description||'Sin descripción'])}/></ResourcePage>
  if(page==='alerts') return <ResourcePage eyebrow="Incidentes" title="Alertas activas" action="Crear alerta"><DataTable headers={['Prioridad','Título','Estado','Mensaje']} rows={alerts.length?alerts.map(a=>[<State key={a.id} value={a.priority}/>,a.title,a.state,a.message]):[[<State key="none" value="success"/>,'Sin alertas activas','—','La plataforma no reporta incidentes']]}/></ResourcePage>
  if(page==='devices') return <ResourcePage eyebrow="Fleet" title="Dispositivos registrados" action="Provisionar dispositivo"><DataTable headers={['Dispositivo','ID','Estado','Firmware']} rows={devices.length?devices.map(d=>[d.name,d.device_id,<State key={d.id} value={d.state}/>,d.firmware_version??'Sin reporte']):[['Sin dispositivos','—',<State key="off" value="offline"/>,'Provisioná el primer ESP32']]}/></ResourcePage>
  if(page==='deployments') return <DeploymentPage values={deployments}/>
  if(page==='markets') return <MarketPage values={quotes}/>
  if(page==='weather') return <WeatherPage value={weather}/>
  return <SettingsPage/>
}

function Dashboard({products,services,alerts,devices,deployments}:{products:Product[];services:Service[];alerts:Alert[];devices:Device[];deployments:Deployment[]}) {
  const operational=services.filter(s=>s.status==='operational').length
  const activity=[...alerts.slice(0,3).map(a=>[a.title,a.message,a.priority]),...deployments.slice(0,3).map(d=>[`${d.workflow}: ${deploymentLabel(d.status)}`,`${d.repository} · ${d.branch}`,d.status==='failure'?'critical':'info'])].slice(0,5)
  return <><section className="hero-panel glass"><div><span className="eyebrow">ESTADO GENERAL</span><h2>Todo bajo control.</h2><p>Una vista única de productos, servicios y dispositivos en tiempo real.</p></div><div className="hero-orbit"><span/><strong>{alerts.length}</strong><small>alertas activas</small></div></section>
    <section className="kpi-grid"><Metric icon="box" label="Productos" value={products.length} detail="configurados"/><Metric icon="server" label="Servicios" value={`${operational}/${services.length}`} detail="operacionales"/><Metric icon="alert" label="Alertas" value={alerts.length} detail="requieren atención"/><Metric icon="activity" label="Dispositivos" value={devices.filter(d=>d.state==='online').length} detail={`${devices.length} registrados`}/></section>
    <section className="dashboard-grid"><div className="panel"><PanelTitle title="Salud de productos" subtitle="Estado agregado por servicio"/><div className="health-list">{products.map(p=><div key={p.id}><span className={`health-icon tone-${p.health}`}><Icon name="box"/></span><div><strong>{p.name}</strong><small>{services.filter(s=>s.product_id===p.id).length} servicios</small></div><State value={p.health}/><i style={{width:p.health==='operational'?'100%':p.health==='unknown'?'12%':'60%'}}/></div>)}</div></div><div className="panel"><PanelTitle title="Actividad reciente" subtitle="Alertas y deployments reales"/><div className="timeline">{activity.length?activity.map(([name,detail,state],index)=><div key={`${name}-${index}`}><span className={`live-dot tone-${state}`}/><div><strong>{name}</strong><small>{detail}</small></div></div>):<p className="muted">Aún no se recibieron eventos.</p>}</div></div></section>
  </>
}

function ResourcePage({eyebrow,title,action,children}:{eyebrow:string;title:string;action:string;children:React.ReactNode}) { return <><div className="section-heading"><div><span className="eyebrow">{eyebrow}</span><h2>{title}</h2></div><button className="primary-action">＋ {action}</button></div><div className="panel table-panel">{children}</div></> }
function DataTable({headers,rows}:{headers:string[];rows:React.ReactNode[][]}) { return <div className="data-table"><div className="table-row table-head">{headers.map(x=><span key={x}>{x}</span>)}</div>{rows.map((row,i)=><div className="table-row" key={i}>{row.map((cell,j)=><span key={j}>{cell}</span>)}</div>)}</div> }
function State({value}:{value:string}) { const label:{[k:string]:string}={operational:'Operacional',degraded:'Degradado',outage:'Caído',unknown:'Sin datos',online:'Online',offline:'Offline',critical:'Crítica',warning:'Warning',info:'Info',success:'OK',failure:'Falló',queued:'En cola',in_progress:'En curso',cancelled:'Cancelado'}; return <span className={`state-chip tone-${value}`}>{label[value]??value}</span> }
function Metric({icon,label,value,detail}:{icon:'box'|'server'|'alert'|'activity';label:string;value:string|number;detail:string}) { return <div className="metric-card"><span><Icon name={icon}/></span><small>{label}</small><strong>{value}</strong><p>{detail}</p></div> }
function PanelTitle({title,subtitle}:{title:string;subtitle:string}) { return <div className="panel-title"><div><h3>{title}</h3><p>{subtitle}</p></div><button>•••</button></div> }
function DeploymentPage({values}:{values:Deployment[]}){return <ResourcePage eyebrow="CI/CD" title="Deployments recientes" action="Configurar webhook"><DataTable headers={['Repositorio','Workflow','Estado','Commit / autor']} rows={values.length?values.map(value=>[`${value.repository} · ${value.branch}`,value.workflow,<State key={value.id} value={value.status}/>,`${value.commit_sha.slice(0,8)} · ${value.author}`]):[['Sin deployments','—',<State key="empty" value="unknown"/>,'Esperando GitHub Actions']]}/></ResourcePage>}
function MarketPage({values}:{values:MarketQuote[]}){
  const bySymbol=new Map<string,MarketQuote>();for(const value of values)if(!bySymbol.has(value.symbol))bySymbol.set(value.symbol,value);const latest=[...bySymbol.values()]
  const names:Record<string,string>={USD_BLUE:'Dólar Blue',BTCUSDT:'Bitcoin',XRPUSDT:'XRP'}
  return <><div className="section-heading"><div><span className="eyebrow">COTIZACIÓN YA</span><h2>Mercados</h2></div><span className="sync-state"><span className={`live-dot ${latest.length?'':'pending'}`}/>{latest.length?'Datos persistidos':'Esperando primera recolección'}</span></div><div className="quote-grid">{latest.map(value=><div className="quote-card" key={value.symbol}><small>{names[value.symbol]??value.symbol}</small><strong>{formatQuote(value)}</strong><span className={value.change_percent<0?'down':''}>{value.change_percent>=0?'+':''}{value.change_percent.toLocaleString('es-AR',{maximumFractionDigits:3})}%</span><div className="sparkline">▁▂▃▂▄▅▄▆▇</div><small>{value.stale?'Dato desactualizado':`Actualizado ${relativeTime(value.observed_at)}`}</small></div>)}</div>{!latest.length?<div className="empty-insight glass"><h2>Preparando mercados</h2><p>El colector se ejecuta al iniciar el backend.</p></div>:null}</>}
function WeatherPage({value}:{value?:Weather}){if(!value)return <div className="empty-insight glass"><span><Icon name="cloud" size={28}/></span><h2>Preparando el clima</h2><p>Open-Meteo se consulta automáticamente al iniciar.</p></div>;return <div className="weather-hero glass"><span><Icon name="cloud" size={52}/></span><div><span className="eyebrow">{value.location} · CÓRDOBA</span><strong>{Math.round(value.temperature_c)}°</strong><p>Sensación {value.apparent_temperature_c.toLocaleString('es-AR',{maximumFractionDigits:1})}° · {weatherDescription(value.weather_code)}</p><small>{value.stale?'Dato desactualizado':`Actualizado ${relativeTime(value.observed_at)}`}</small></div><dl><div><dt>Mínima</dt><dd>{formatDegree(value.daily_min_c)}</dd></div><div><dt>Máxima</dt><dd>{formatDegree(value.daily_max_c)}</dd></div><div><dt>Lluvia</dt><dd>{value.precipitation_probability_percent==null?'—':`${Math.round(value.precipitation_probability_percent)}%`}</dd></div></dl></div>}
function SettingsPage(){return <div className="settings-grid"><div className="panel"><PanelTitle title="Plataforma" subtitle="Configuración efectiva desde YAML"/><label>Zona horaria<input value="America/Argentina/Cordoba" readOnly/></label><label>Perfil predeterminado<input value="operations" readOnly/></label><label>Intervalo del dashboard<input value="5 minutos" readOnly/></label></div><div className="panel architecture-card"><span><Icon name="check"/></span><h3>Arquitectura lista</h3><p>La fuente de verdad permanece versionada en YAML. Los cambios administrativos serán persistidos con revisión y auditoría.</p><code>contracts/config/platform.example.yaml</code></div></div>}

function Login({onAuthenticated,backendOnline}:{onAuthenticated:()=>void;backendOnline:boolean}){
  const[email,setEmail]=useState(''),[password,setPassword]=useState(''),[error,setError]=useState(''),[loading,setLoading]=useState(false)
  const submit=async(event:FormEvent)=>{event.preventDefault();setLoading(true);setError('');try{const result=await login(email,password);localStorage.setItem('desk_access_token',result.access_token);onAuthenticated()}catch{setError('No se pudo iniciar sesión. Revisá las credenciales.')}finally{setLoading(false)}}
  return <main className="login-shell"><div className="ambient ambient-left"/><div className="ambient ambient-right"/><form className="login-card glass" onSubmit={submit}><span className="brand-mark"><Icon name="activity"/></span><span className="eyebrow">DESK MONITOR PLATFORM</span><h1>Centro de monitoreo</h1><p>Ingresá para acceder a servicios, cotizaciones, clima y dispositivos.</p><label>Correo electrónico<input type="email" autoComplete="username" required value={email} onChange={event=>setEmail(event.target.value)}/></label><label>Contraseña<input type="password" autoComplete="current-password" minLength={8} required value={password} onChange={event=>setPassword(event.target.value)}/></label>{error?<div className="login-error">{error}</div>:null}<button className="primary-action" disabled={loading}>{loading?'Ingresando…':'Ingresar'}</button><small><span className={`live-dot ${backendOnline?'':'pending'}`}/>{backendOnline?'Backend operacional':'Backend no disponible'}</small></form></main>
}

function formatQuote(value:MarketQuote){if(value.symbol==='USD_BLUE')return new Intl.NumberFormat('es-AR',{style:'currency',currency:'ARS',maximumFractionDigits:2}).format(value.last);return `${new Intl.NumberFormat('es-AR',{maximumFractionDigits:value.last<10?4:2}).format(value.last)} ${value.quote_asset}`}
function formatDegree(value:number|null){return value==null?'—':`${Math.round(value)}°`}
function relativeTime(value:string){const minutes=Math.max(0,Math.round((Date.now()-new Date(value).getTime())/60000));return minutes<1?'ahora':minutes<60?`hace ${minutes} min`:`hace ${Math.round(minutes/60)} h`}
function weatherDescription(code:number){if(code===0)return'Despejado';if(code<=3)return'Parcialmente nublado';if(code<=48)return'Niebla';if(code<=67)return'Lluvia';if(code<=77)return'Nieve';if(code<=82)return'Chaparrones';return'Tormentas'}
function deploymentLabel(value:string){return({queued:'En cola',in_progress:'En curso',success:'Completado',failure:'Falló',cancelled:'Cancelado'} as Record<string,string>)[value]??value}
