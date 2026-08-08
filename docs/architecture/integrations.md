# Integraciones y colectores

## Puerto canónico

```go
type Collector interface {
    Name() string
    Interval() time.Duration
    Collect(context.Context) (CollectResult, error)
}
```

`CollectResult` contiene `source`, `observed_at`, observaciones canónicas y warnings
no fatales. Una observación identifica `product_key` y opcionalmente `service_key`,
su clase (`service_status`, `market_quote`, `weather` o `metric`) y datos tipados.
El runner controla timeout, jitter de ±10 %, circuit breaker y persistencia; el
adaptador no abre transacciones ni publica MQTT.

Una ejecución es exitosa si todos los campos requeridos se normalizan. Campos
opcionales inválidos producen warnings. No se actualiza el estado actual con una
respuesta parcial que omita un required. Tras tres fallos consecutivos se abre una
alerta warning; tras seis, critical. El primer éxito posterior resuelve esa alerta.

## Cotización Ya

- API base configurable y sin credenciales en v1.
- Dólar blue se normaliza con compra/venta; BTC y XRP usan pares contra USDT.
- Cada ejecución agrega registros históricos, nunca sobrescribe.
- `change_percent` canónico se calcula contra la observación previa almacenada. El
  porcentaje entregado por el proveedor se conserva como metadata para auditoría.
- División por cero o dato previo ausente produce variación `null`, no cero.
- Duplicados de provider/symbol/observed_at son idempotentes.

La API pública vigente declara endpoints de Argentina y activos cripto normalizados
desde Binance: <https://cotizacionya.ar/api/docs>.

## Open-Meteo

La ubicación default es Río Cuarto (`-33.1307`, `-64.3499`) y la zona horaria
`America/Argentina/Cordoba`, siempre configurables. Se solicitan condiciones
actuales de temperatura, sensación, humedad, código WMO y viento, más mínima,
máxima y probabilidad de precipitación diaria. Código WMO se traduce en backend a
un icono semántico estable.

Se persiste una observación por timestamp del proveedor y se marca stale después de
dos intervalos sin éxito. Referencia: <https://open-meteo.com/en/docs>.

## HTTP JSON genérico / Potrerito

El adaptador solo hace GET JSON. Cada mapping declara destino, entidad, campo,
JSONPath, tipo, required, escala y default opcional. Los únicos destinos v1 son:

- `service_status`: `status`, `latency_ms`, `reason`, `observed_at`.
- `product_status`: `status`, `summary`, `observed_at`.
- `metric`: `value`, `unit`, `label`, `observed_at`.

Estados de terceros se convierten mediante una tabla declarativa hacia estados
canónicos. Un mapping no puede crear topics, SQL, scripts ni pantallas. Potrerito
queda deshabilitado hasta configurar una URL real, allowlist y token. Nunca existe
conexión a su PostgreSQL.

## GitHub

El webhook oficial acepta `ping` y `workflow_run`; cualquier otro evento devuelve
202 sin mutar dominio. La firma se verifica antes de parsear. `X-GitHub-Delivery`
es la clave idempotente. Se normalizan repositorio completo, branch, workflow,
status/conclusion, SHA, mensaje, autor, timestamps y URL.

El endpoint Actions recibe el DTO canónico OpenAPI y exige `Idempotency-Key` y bearer.
Mapeo: queued→queued, requested/in_progress→in_progress, success→success,
failure/timed_out/action_required→failure, cancelled/skipped→cancelled. Un failure
abre alerta warning del producto asociado; success puede emitir notificación success
y resolver la alerta de deployment correspondiente.

Referencia de `workflow_run`:
<https://docs.github.com/en/webhooks/webhook-events-and-payloads#workflow_run>.

## Uptime Kuma

El endpoint recibe el preset JSON con `monitor`, `heartbeat` y `msg`, protegido por
bearer e `Idempotency-Key`. `heartbeat.status=1` se normaliza como operational y
`0` como outage; ping se guarda como latencia. La asociación monitor→servicio es
configurable por ID externo.

Outage abre o actualiza una alerta por fingerprint estable; volver a operational la
resuelve. Ambos cambios crean outbox events. Un monitor desconocido se acepta y se
registra en el log estructurado, pero no modifica servicios hasta tener asociación.

## Scheduler y concurrencia

V1 ejecuta una instancia por colector; no solapa runs del mismo nombre. Un trigger
manual ya en curso devuelve 202 y reutiliza el run activo. Backoff exponencial tiene
tope de cinco minutos y respeta `Retry-After`. Al migrar a varias réplicas, Redis
provee leases con fencing token; no se habilitan réplicas mientras SQLite sea el
writer.
