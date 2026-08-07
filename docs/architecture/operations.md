# Configuración y operación

## Precedencia

1. Los defaults no secretos se cargan desde YAML tipado.
2. Variables de entorno aplican overrides de despliegue y secretos.
3. La base de datos contiene configuración funcional editable.

El YAML siembra registros por stable key solo cuando no existen. Una vez creado,
el registro de DB es autoritativo. Cada cambio incrementa `revision`, genera una
auditoría e invalida cache/perfiles. La aplicación falla al arrancar ante claves
desconocidas, valores inválidos o referencias rotas.

Los valores operacionales iniciales serán:

| Configuración | Default YAML |
|---|---:|
| Cotizaciones | 5 minutos |
| Clima | 15 minutos |
| HTTP genérico | 1 minuto |
| Pantalla normal | 8 segundos |
| Pantallas por snapshot | 12 |
| Histórico mercado/deploy/alerta | 365 días |
| Clima/telemetría/webhooks/runs | 30 días |

## Compose productivo

Servicios previstos para la etapa Docker: Caddy, frontend estático, backend,
Redis, Mosquitto y Prometheus. Solo Caddy publica HTTP/HTTPS. MQTT externo usa
WebSocket seguro a través de Caddy; backend y broker se comunican por la red
interna. Volúmenes separados conservan SQLite, claves, Redis y estado de Mosquitto.

`docker compose up -d` debe iniciar en modo local sin secretos inseguros embebidos.
El backend genera claves internas persistentes cuando faltan y el primer admin se
crea mediante `make bootstrap-admin`. En producción se configura dominio y secretos
externos antes de exponer el host.

## Salud y observabilidad

- Liveness confirma proceso y event loop.
- Readiness exige configuración, migraciones, DB y claves válidas.
- `/api/v1/health` expone únicamente estado general y timestamp.
- Liveness/readiness detallados viven en el listener interno y Caddy no los publica.
- `/metrics` expone latencia HTTP, requests, colectores, cache, outbox, MQTT,
  dispositivos y estado de integraciones.
- Logs JSON incluyen timestamp UTC, level, service, request/correlation ID y actor.

## Backups

El comando `backup` usa la API online de SQLite, incluye manifest con schema version
y checksum, escribe primero a un archivo temporal y realiza rename atómico. La
retención y el destino son configurables. Restaurar requiere parar el escritor,
verificar checksum, ejecutar migraciones pendientes y realizar un smoke test. Un
backup no se considera válido hasta que una restauración automatizada lo pruebe.

## Respuesta a fallos

- Redis: cache bypass y SSE local; health degradado.
- MQTT: outbox con backoff; no se pierden alertas y REST conserva estado.
- Fuente externa: último dato marcado stale, circuit breaker y alerta configurable.
- Disco/SQLite: readiness falla y se detienen colectores; no se descartan escrituras.
- Payload incompatible: se rechaza con error estable, métrica y delivery auditada.
