# Desk Monitor Backend

Backend independiente de Desk Monitor Platform. El primer corte de la etapa Backend
implementa configuración tipada, migraciones explícitas, persistencia portable,
productos, perfiles, dispositivos y autenticación Ed25519.

## Comandos

```bash
go test ./...
go vet ./...
go run ./cmd/desk-monitor version
go run ./cmd/desk-monitor server
```

Para crear el primer administrador se proporcionan una sola vez
`DESK_BOOTSTRAP_ADMIN_EMAIL` y `DESK_BOOTSTRAP_ADMIN_PASSWORD`. No existen
credenciales por defecto; si el usuario ya existe el bootstrap no modifica su
password.

El listener público usa `:8080`; health detallado y métricas viven en el listener
interno `:9090`. Ambos se configuran mediante environment.

## Implementado en el corte 4.1

- YAML estricto con referencias de secretos y seeds idempotentes por stable key.
- Migraciones SQL embebidas para SQLite y PostgreSQL; GORM no altera el esquema.
- SQLite en WAL con un único escritor y ruta PostgreSQL mediante `DATABASE_URL`.
- Argon2id para passwords/secrets y access JWT Ed25519 con `kid` y audience.
- Refresh token rotativo en cookie HttpOnly/SameSite Strict y detección de reuse.
- REST funcional para auth, products, profiles y devices, protegido por roles.
- Errores RFC 9457, Request ID, panic recovery y logging JSON estructurado.

Dashboard, outbox/MQTT, servicios, alertas, webhooks y colectores se incorporan en
los siguientes cortes de la etapa 4; no hay handlers placeholder para esas rutas.

## Dependencias internas

```text
cmd -> bootstrap -> application -> domain
                         |          ^
                         v          |
                       ports <--- adapters
```

`domain` y `application` no pueden importar paquetes bajo `adapters` o `platform`.
