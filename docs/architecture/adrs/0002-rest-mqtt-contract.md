# ADR 0002: REST autoritativo y MQTT para eventos

- Estado: Accepted
- Fecha: 2026-08-06

## Decisión

REST entrega dashboard/config completos, versionados y cacheables mediante ETag.
MQTT entrega alertas, notificaciones, comandos, presencia, telemetría y ACK. Un
evento MQTT nunca es la única copia del estado durable.

Alertas y comandos usan QoS 1, ID idempotente y expiración. El ESP32 interrumpe la
rotación ante `critical`, y al reconectar restaura el estado mediante REST. MQTT
externo viaja como WSS terminado por Cloudflare y atraviesa el Nginx del host hacia
el listener WebSocket de Mosquitto enlazado exclusivamente a loopback.

## Consecuencias

El firmware tolera eventos duplicados y desconexiones sin reconstruir estado desde
un log MQTT. El backend puede cambiar fuentes y reglas sin actualizar dispositivos.
