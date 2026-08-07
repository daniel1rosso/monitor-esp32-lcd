# Seguridad

## Actores y credenciales

| Actor | Autenticación | Autorización |
|---|---|---|
| Admin web | email + Argon2id, access JWT 15 min, refresh 7 días | rol `admin` |
| Viewer web | igual al admin | rol `viewer`, sin mutaciones |
| ESP32 | `device_id + secret` por TLS, JWT 15 min | subject y audience de dispositivo |
| ESP32 MQTT | usuario/password MQTT únicos | ACL exacta por device ID |
| GitHub webhook | `X-Hub-Signature-256` HMAC-SHA256 | evento permitido y delivery ID |
| GitHub Actions | bearer dedicado | scope `events:github` |
| Uptime Kuma | bearer dedicado | scope `events:uptime` |

Los JWT se firman con Ed25519 e incluyen `iss`, `aud`, `sub`, `jti`, `iat`, `nbf`,
`exp` y `kid`. Las claves se generan una vez en un volumen protegido o se montan
desde secretos externos. La rotación admite varias claves públicas activas.

El secreto de dispositivo se muestra una sola vez, se almacena con Argon2id y
puede revocarse por generación. La credencial MQTT es distinta. Mosquitto Dynamic
Security concede lectura de alerts/notifications/commands/global y escritura de
status/telemetry/acks únicamente para el propio device ID.

## Controles HTTP

- TLS obligatorio fuera del modo local; CORS restringido al origen administrativo.
- Refresh token rotativo, almacenado como hash; reutilizar uno revocado invalida la
  familia completa.
- Access token en memoria del navegador; refresh en cookie HttpOnly, Secure y
  SameSite=Strict. Mutaciones verifican `Origin`.
- Rate limiting separado para login, canje de dispositivo y webhooks.
- Errores RFC 9457 sin stack traces, SQL ni secretos.
- Tamaño máximo por ruta, JSON estricto, timeout de lectura/escritura y request ID.
- Webhooks se verifican sobre los bytes originales antes de decodificar y se
  deduplican por delivery ID o hash estable.

## Colector HTTP genérico

Solo soporta GET JSON en v1. El operador define una allowlist explícita de host,
puerto y CIDR; esto permite una API interna como Potrerito sin habilitar SSRF libre.
Se bloquean redirects fuera de la allowlist, credenciales en URL, respuestas sobre
1 MiB y tipos de contenido inesperados. Los secretos se referencian por nombre de
variable de entorno y nunca se devuelven al frontend.

## Auditoría y privacidad

Creación/revocación de credenciales, cambios de configuración, acciones sobre
alertas y reintentos manuales producen `audit_log`. Logs y métricas no incluyen
passwords, tokens, headers de autenticación ni payloads completos de terceros. El
raw body de webhook es opcional, cifrado si se conserva y se elimina a los 30 días.

## Threat model mínimo

Se prueban replay de webhook, JWT expirado o con audience errónea, refresh robado,
escalada viewer/admin, device ID ajeno en REST/MQTT, payload MQTT duplicado,
inyección JSONPath, SSRF, descompresión/bodies excesivos y filtración en logs.

