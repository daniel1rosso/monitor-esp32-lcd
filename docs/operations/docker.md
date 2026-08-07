# Operación Docker

## Stack local

```bash
docker compose up -d --build
docker compose ps
curl http://127.0.0.1:9096/api/v1/health
```

Docker publica únicamente tres puertos sobre loopback: frontend `9095`, API
`9096` y MQTT WebSocket `9097`. Nginx está instalado en el host, escucha HTTP/80
y publica el dominio; Cloudflare termina HTTPS/WSS. Redis, Mosquitto TCP,
Prometheus y el listener de management permanecen en la red Docker `internal`.

| Servicio | Responsabilidad | Persistencia |
|---|---|---|
| frontend | assets estáticos SPA | ninguna |
| backend | API pública, probes y métricas | `/data` |
| redis | cache/coordinación | AOF |
| mosquitto | MQTT y Dynamic Security | estado/credenciales |
| prometheus | scraping y series | TSDB 15 días |

`docker compose down` conserva los datos. No usar `down -v` en un entorno con
datos reales.

## Producción

Crear el archivo privado de variables y completar sus credenciales:

```bash
cp .env.production.example .env.production
```

Validar y desplegar:

```bash
docker compose --env-file .env.production \
  -f docker-compose.yml -f docker-compose.prod.yml config --quiet

docker compose --env-file .env.production \
  -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

También están disponibles `make prod-config` y `make prod-up`.

Instalar la configuración del Nginx anfitrión:

```bash
sudo cp deploy/nginx/monitor.danielalbertorosso.com.ar.conf /etc/nginx/conf.d/desk-monitor.conf
sudo nginx -t
sudo systemctl reload nginx
```

Cloudflare debe resolver `monitor.danielalbertorosso.com.ar` hacia el servidor,
usar el proxy naranja, redirigir HTTP público a HTTPS y conectarse al origen por
HTTP. Los dispositivos utilizan siempre estas URLs públicas:

```text
https://monitor.danielalbertorosso.com.ar
wss://monitor.danielalbertorosso.com.ar/mqtt
```

El salto Cloudflare-origen queda sin cifrar por decisión del despliegue actual.
El firewall debería aceptar HTTP/80 únicamente desde los rangos de Cloudflare o
migrarse posteriormente a Cloudflare Tunnel.

## Secretos

`.env.production` está ignorado por Git. La plantilla versionada no contiene
secretos. Para el primer arranque de un volumen nuevo se completan conjuntamente
`DESK_BOOTSTRAP_ADMIN_EMAIL` y `DESK_BOOTSTRAP_ADMIN_PASSWORD`.

Mosquitto inicializa Dynamic Security una sola vez. Si no se proporciona
`MOSQUITTO_ADMIN_PASSWORD`, genera y persiste una clave aleatoria en
`mosquitto_data`. Cambiar la variable después no rota una instalación existente.

## Diagnóstico

```bash
docker compose ps
docker compose logs backend frontend mosquitto redis prometheus
docker compose exec backend desk-monitor probe
curl http://127.0.0.1:9095/
curl http://127.0.0.1:9096/api/v1/health
sudo nginx -t
```

Readiness detallado, métricas y MQTT TCP no se publican por Nginx. Los
healthchecks esperan a Redis y Mosquitto antes de iniciar el backend.

## Backups

SQLite reside en el volumen `backend_data`. Copiar el archivo mientras existe un
writer no constituye un backup válido. El procedimiento productivo debe usar la
API online de SQLite, generar checksum y comprobar periódicamente una restauración.
