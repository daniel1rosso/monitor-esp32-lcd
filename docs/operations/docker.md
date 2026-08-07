# Operación Docker

## Stack local

```bash
docker compose up -d --build
docker compose ps
curl http://localhost:8180/api/v1/health
```

Caddy es el único servicio HTTP publicado. Redis, Mosquitto TCP, Prometheus y el
listener de management del backend permanecen en una red Docker marcada `internal`.
MQTT para dispositivos usa WebSocket en `/mqtt` a través de Caddy.

Servicios:

| Servicio | Responsabilidad | Persistencia |
|---|---|---|
| caddy | borde, headers, compresión, HTTPS/WSS | certificados/config |
| frontend | assets estáticos SPA | ninguna |
| backend | API pública, probes y métricas | `/data` |
| redis | cache/coordinación | AOF |
| mosquitto | MQTT y Dynamic Security | estado/credenciales |
| prometheus | scraping y series | TSDB 15 días |

`docker compose down` conserva datos. No usar `down -v` en un entorno con datos.

## Configuración

Los defaults funcionan sin `.env`. Para personalizar:

```bash
cp .env.example .env
docker compose config --quiet
docker compose up -d --build
```

En producción, establecer un dominio en `DESK_SITE_ADDRESS`, cargar secretos reales
y usar el override que publica HTTP, HTTPS y HTTP/3:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

El Compose base enlaza 8180 únicamente a loopback. Para pruebas desde un ESP32 en
la LAN se puede definir temporalmente `DESK_BIND_ADDRESS=0.0.0.0`; en producción los
dispositivos deben usar el dominio HTTPS/WSS.

Mosquitto inicializa Dynamic Security una sola vez. Si no se proporciona
`MOSQUITTO_ADMIN_PASSWORD`, guarda una clave aleatoria con permisos 0600 dentro del
volumen `mosquitto_data`. Cambiar la variable después no rota una instalación ya
inicializada; la rotación será responsabilidad del caso de uso de dispositivos.

## Desarrollo

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
```

El override usa Air para Go y Vite con HMR. Los servicios de infraestructura son los
mismos que en producción para evitar entornos ficticios.

## Diagnóstico

```bash
docker compose ps
docker compose logs backend caddy mosquitto redis prometheus
docker compose exec backend desk-monitor probe
docker compose exec prometheus promtool check config /etc/prometheus/prometheus.yml
```

El health público solo revela `status` y timestamp. Readiness detallado y métricas
no pasan por Caddy. Los healthchecks de Compose esperan a Redis/Mosquitto antes del
backend y al backend/frontend antes del proxy.

## Datos y backups

Los seis volúmenes tienen nombres estables bajo el proyecto `desk-monitor`. Copiar
el archivo SQLite mientras exista un writer no será un backup válido. El comando de
backup online y su prueba de restauración se implementan junto con persistencia en
la etapa Backend; hasta entonces el volumen backend no contiene datos funcionales.
