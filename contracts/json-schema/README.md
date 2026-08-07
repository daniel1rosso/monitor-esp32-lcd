# Contratos JSON del dispositivo

Los schemas usan JSON Schema 2020-12. `schema_version` empieza en `1`; campos
opcionales pueden agregarse sin incrementar la versión mayor. El firmware rechaza
un documento inválido, omite pantallas con tipo desconocido y conserva el último
snapshot válido hasta `expires_at`.

| Contrato | Uso |
|---|---|
| [dashboard.schema.json](dashboard.schema.json) | Snapshot REST de pantallas |
| [device-config.schema.json](device-config.schema.json) | Config REST efectiva |
| [mqtt-envelope.schema.json](mqtt-envelope.schema.json) | Mensajes MQTT |

Los ejemplos de `fixtures/` son datos de prueba compartidos por backend y firmware.
Límites v1: 12 pantallas, documento dashboard de 64 KiB, MQTT de 8 KiB, título de
48 caracteres y duración entre 3 y 60 segundos.

