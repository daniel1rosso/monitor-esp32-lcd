# ADR 0003: SQLite portable y outbox transaccional

- Estado: Accepted
- Fecha: 2026-08-06

## Decisión

SQLite WAL es la base inicial y existe un solo proceso escritor. Las migraciones SQL
son explícitas, versionadas, embebidas y se ejecutan con lock al arrancar. GORM no
usa AutoMigrate en producción. IDs ULID, UTC y tipos portables mantienen una ruta
probada hacia PostgreSQL.

Cada evento externo se escribe en `outbox_events` en la misma transacción que el
cambio de dominio. Un dispatcher publica y reintenta con backoff; los consumidores
deduplican por `event_id`.

## Consecuencias

No existe una ventana donde la DB confirme una alerta pero MQTT la pierda. Varias
réplicas solo se habilitarán después de migrar a PostgreSQL y activar locks
distribuidos estrictos.

