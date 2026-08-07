# Protocolo MQTT

MQTT complementa REST y no reemplaza snapshots. Todos los payloads validan contra
[`mqtt-envelope.schema.json`](../json-schema/mqtt-envelope.schema.json), pesan como
máximo 8 KiB y usan UTF-8.

## Semántica

| Topic | Dirección del dispositivo | QoS | Retained | Uso |
|---|---|---:|---:|---|
| `desk/device/{id}/alerts` | subscribe | 1 | no | interrupciones y resolución |
| `desk/device/{id}/notifications` | subscribe | 1 | no | mensajes no durables |
| `desk/device/{id}/commands` | subscribe | 1 | no | refresh/restart/identify |
| `desk/device/{id}/status` | publish | 1 | sí | online y Last Will offline |
| `desk/device/{id}/telemetry` | publish | 0 | no | heartbeat y capacidades |
| `desk/device/{id}/acks` | publish | 1 | no | resultado de comandos |
| `desk/global/events` | subscribe | 1 | no | eventos relevantes globales |

El cliente usa sesión persistente con `clean_start=false`, expiración de sesión
configurable y Last Will antes de publicar `online`. Descarta eventos vencidos y
mantiene un LRU de al menos 64 `event_id` para deduplicar QoS 1.

Una alerta `critical` pausa la pantalla actual, entra en una cola ordenada por
prioridad y luego FIFO, se muestra hasta `duration_ms` o `expires_at`, y restaura el
índice previo. `alert.resolved` elimina una alerta pendiente o visible. Un comando
siempre produce ACK, incluido `unsupported`.

## ACL

- Backend: publish a todos los topics de salida y subscribe a status/telemetry/acks.
- Dispositivo: acceso exacto a su `{id}`; nunca se permiten wildcards suministrados
  por el cliente.
- Dispositivo: subscribe de solo lectura a `desk/global/events`.
- Usuarios anónimos: sin acceso.

