# Arquitectura

## Contexto y límites

Desk Monitor Platform atiende una organización por instalación y múltiples
productos, servicios, perfiles y dispositivos. Los productos iniciales son
`potrerito`, `freshcode` y `facturas`; son datos configurables, no conceptos del
firmware.

```mermaid
C4Context
  title Desk Monitor Platform
  Person(operator, "Operador", "Administra y observa")
  System(device, "ESP32 Desk Monitor", "Renderiza pantallas tipadas")
  System(platform, "Desk Monitor Platform", "Normaliza, decide y distribuye")
  System_Ext(sources, "Fuentes externas", "Cotización Ya, Open-Meteo, Potrerito")
  System_Ext(events, "Productores de eventos", "GitHub, Uptime Kuma")
  Rel(operator, platform, "HTTPS/SSE")
  Rel(platform, sources, "HTTPS polling")
  Rel(events, platform, "Webhooks HTTPS")
  Rel(device, platform, "HTTPS + MQTT/WSS")
```

## Contenedores de runtime

```mermaid
flowchart LR
  Browser -->|HTTPS| Cloudflare
  ESP -->|HTTPS / MQTT over WSS| Cloudflare
  GitHub -->|HMAC webhook| Cloudflare
  Kuma[Uptime Kuma] -->|Bearer webhook| Cloudflare
  Cloudflare -->|HTTP :80| Nginx[Host Nginx]
  Nginx -->|127.0.0.1:9095| Frontend
  Nginx -->|127.0.0.1:9096| API[Go API + Scheduler]
  Nginx -->|127.0.0.1:9097| Mosquitto
  API --> SQLite[(SQLite WAL)]
  API --> Redis[(Redis)]
  API --> Mosquitto[(Mosquitto)]
  API --> Sources[External APIs]
  Prometheus --> API
```

Nginx es el borde HTTP del host y los tres puertos de aplicación están enlazados
exclusivamente a loopback. Cloudflare termina HTTPS/WSS. El listener MQTT TCP solo
es visible en la red de Compose. El backend es un único proceso en v1 para evitar
escritores SQLite distribuidos.

## Componentes del backend

```mermaid
flowchart TD
  HTTP[HTTP / SSE / Middleware] --> UC[Application Use Cases]
  Scheduler[Collector Scheduler] --> UC
  Ingestors[Webhook Ingestors] --> UC
  UC --> Domain[Domain + Domain Events]
  UC --> Ports[Repository / Cache / Event / Clock ports]
  Ports --> Gorm[GORM repositories]
  Ports --> RedisAdapter[Redis adapter]
  Ports --> Outbox[Transactional outbox]
  Outbox --> MQTT[MQTT publisher]
  Outbox --> SSE[SSE broker]
  Collectors[Collectors] --> Sources[External clients]
```

Las dependencias siempre apuntan hacia `domain` y `application`. Los handlers
traducen DTOs; nunca contienen reglas de negocio. Los repositorios devuelven
entidades de dominio y encapsulan GORM. La composición de dependencias es manual
en un composition root.

## Flujos principales

### Snapshot del dispositivo

1. El dispositivo canjea `device_id + secret` por un JWT corto.
2. Solicita dashboard/config usando `If-None-Match` cuando posee una revisión.
3. El caso de uso obtiene perfil, overrides, estado canónico y alertas activas.
4. Los screen providers generan una lista determinística y limitada.
5. El dispositivo valida `schema_version`, almacena el snapshot y rota localmente.

### Alerta durable

1. Un colector, webhook o acción administrativa cambia estado dentro de una
   transacción.
2. La misma transacción crea/actualiza la alerta y escribe un `outbox_event`.
3. El dispatcher publica QoS 1 y registra el intento.
4. MQTT puede duplicar; `event_id` hace idempotente al dispositivo.
5. Si MQTT estuvo caído, el outbox reintenta; REST sigue mostrando la alerta activa.

### Centro web

React Query obtiene recursos REST. SSE solo anuncia eventos e invalidaciones; el
frontend vuelve a consultar la fuente autoritativa. Redis Pub/Sub permite que el
diseño pueda crecer a varias réplicas, pero una caída de Redis no invalida la DB.

## Reglas de disponibilidad

- SQLite, configuración inválida o claves JWT ausentes impiden readiness.
- Redis caído produce estado `degraded`; lecturas DB y un proceso único continúan.
- MQTT caído produce estado `degraded`; el outbox retiene los eventos.
- Una fuente externa caída afecta únicamente su colector y puede abrir su circuit
  breaker.
- Todas las llamadas externas tienen timeout, límite de bytes, retry con jitter y
  métricas por resultado.
