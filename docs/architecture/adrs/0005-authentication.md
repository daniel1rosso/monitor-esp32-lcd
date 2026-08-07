# ADR 0005: Identidades web, dispositivo y MQTT separadas

- Estado: Accepted
- Fecha: 2026-08-06

## Decisión

Usuarios locales usan Argon2id, JWT Ed25519 corto y refresh rotativo. Dispositivos
canjean un secret manual hasheado por JWT de audience específica. MQTT utiliza otra
credencial individual administrada por Mosquitto Dynamic Security.

Viewer solo lee; admin muta. GitHub nativo usa HMAC, Actions y Uptime Kuma usan
bearers con scope. No existen usuarios, passwords ni secrets por defecto.

## Consecuencias

Revocar MQTT no invalida sesiones REST y viceversa. Una credencial filtrada tiene
alcance acotado y cada acción sensible es auditable.

