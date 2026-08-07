# Desk Monitor Platform

Desk Monitor Platform es una plataforma multi-producto para convertir información
operacional, comercial y técnica en pantallas compactas destinadas a dispositivos
ESP32. Las fuentes externas pertenecen al backend: el dispositivo solo entiende un
contrato versionado de pantallas, alertas y comandos.

## Estado del proyecto

Las etapas 1 (**Blueprint**), 2 (**Scaffold**) y 3 (**Docker**) están implementadas.
El backend cubre autenticación, productos, perfiles, dispositivos, servicios,
alertas, outbox/MQTT y snapshots efectivos. También están disponibles el centro
administrativo, el simulador 172×320 y el runtime contractual/LVGL del firmware.
El avance y los límites de cada entrega están en [roadmap.md](docs/roadmap.md).

## Índice

- [Arquitectura](docs/architecture/README.md)
- [Modelo de datos](docs/architecture/data-model.md)
- [Integraciones y colectores](docs/architecture/integrations.md)
- [Seguridad](docs/architecture/security.md)
- [Configuración y operación](docs/architecture/operations.md)
- [Decisiones arquitectónicas](docs/architecture/adrs/README.md)
- [OpenAPI 3.1](contracts/openapi/openapi.yaml)
- [Contratos JSON](contracts/json-schema/README.md)
- [Contrato MQTT](contracts/mqtt/asyncapi.yaml)
- [Diseño de la pantalla](docs/product/device-ui.md)
- [Roadmap y gates](docs/roadmap.md)

## Gate local

```bash
make check
```

## Inicio local

```bash
docker compose up -d --build
curl http://127.0.0.1:9096/api/v1/health
```

El frontend queda en `127.0.0.1:9095`, la API en `127.0.0.1:9096` y MQTT WebSocket
en `127.0.0.1:9097`. En producción, el Nginx instalado en el host publica esas
rutas bajo `monitor.danielalbertorosso.com.ar`. Para habilitar el acceso
administrativo inicial, definir
`DESK_BOOTSTRAP_ADMIN_EMAIL` y `DESK_BOOTSTRAP_ADMIN_PASSWORD` antes del primer
arranque del volumen de datos.

No se requieren credenciales inseguras para iniciar localmente: Mosquitto genera
una clave administrativa aleatoria dentro de su volumen y ningún servicio de datos
publica puertos al host. Ver [operación Docker](docs/operations/docker.md).

## Principios no negociables

- `backend`, `frontend` y `firmware` serán productos independientes.
- El dominio no depende de Gin, GORM, Redis, Mosquitto ni APIs externas.
- REST entrega snapshots autoritativos; MQTT entrega eventos inmediatos.
- SQLite es el almacenamiento inicial, con migraciones y repositorios compatibles
  con PostgreSQL.
- Secretos y credenciales nunca se guardan en YAML ni en el repositorio.
- Los contratos públicos se modifican de forma compatible o mediante una nueva
  versión explícita.
