# Roadmap y gates

## 1. Blueprint — completada

Entregables: arquitectura, ADRs, modelo ER, seguridad, operación, OpenAPI, JSON
Schema y protocolo MQTT. Gate: todos los contratos validan y no existen decisiones
contradictorias.

## 2. Estructura

Crear `backend`, `frontend` y `firmware` independientes, más configuración raíz,
CI, linters y tests mínimos. Gate: cada proyecto compila/testea aislado y la raíz
no contiene lógica de aplicación.

## 3. Docker — completada

Dockerfiles multi-stage, Compose, Nginx de host, Redis, Mosquitto Dynamic Security,
Prometheus, healthchecks, volúmenes, `.env.example`, Makefile y desarrollo con Air.
Gate: `docker compose up -d`, reinicios y persistencia verificados.

## 4. Backend — en progreso

Secuencia interna: configuración/migraciones/health; auth/productos; dispositivos y
perfiles; dashboard/outbox/MQTT; colectores/webhooks; administración y hardening.
Gate: OpenAPI implementado, pruebas unitarias/integración y compatibilidad SQLite /
PostgreSQL.

Estado interno:

- 4.1 configuración, migraciones, auth, productos, perfiles y dispositivos: completado.
- 4.2 servicios, alertas, outbox/MQTT y Dynamic Security: completado.
- 4.3 dashboard y configuración efectiva del dispositivo: completado.
- 4.4 colectores, webhooks y datos históricos: completado.
- 4.5 administración restante, hardening y gate OpenAPI completo: pendiente.

## 5. Frontend — centro visual completado

Shell responsive; login y renovación de sesión; Dashboard/actividad; Products,
Alerts, Deployments, Markets, Weather, Devices, Settings y simulador LCD. React
Query consume exclusivamente la API real y distingue datos ausentes de errores.
Gate actual: lint, TypeScript, build y Vitest. Quedan como hardening las mutaciones
administrativas completas y Playwright.

## 6. Firmware contractual — completada

ESP-IDF para Waveshare ESP32-C6-LCD-1.47: modelo y parser JSON acotado, snapshot,
máquina de rotación con interrupción/reanudación crítica, pruebas host-side y
renderer LVGL desacoplado. La inicialización del BSP, red, REST y MQTT físicos
requiere la placa y constituye el siguiente gate de hardware.

La actualización OTA no forma parte de v1. Los contratos reservan versión de
firmware y el comando `restart`, pero no incluyen descarga, firma ni particionado
OTA hasta completar un ADR y threat model específicos.

## 7. Producción

TLS/WSS, backups restaurados, rotaciones, métricas, recursos, escaneo y release
reproducible. Ninguna etapa avanza con tests rojos, documentación divergente o una
dependencia de infraestructura dentro del dominio.
