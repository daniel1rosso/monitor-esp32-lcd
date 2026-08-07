# ADR 0001: Clean Architecture y DDD liviano

- Estado: Accepted
- Fecha: 2026-08-06

## Decisión

El backend se divide en dominio, aplicación, puertos y adaptadores. El dominio
contiene agregados, value objects, políticas y eventos sin imports de frameworks.
Application orquesta transacciones mediante interfaces. Gin, GORM, Redis, MQTT y
clientes HTTP son adaptadores conectados en un composition root manual.

Se evitan capas ceremoniales: un agregado pequeño puede compartir paquete con sus
políticas y no se crean interfaces sin una frontera real o sustitución en tests.

## Consecuencias

Los DTO HTTP no cruzan hacia dominio, GORM no define las entidades públicas y los
colectores devuelven resultados canónicos. Agregar una fuente o base de datos no
modifica reglas de pantalla ni firmware.

