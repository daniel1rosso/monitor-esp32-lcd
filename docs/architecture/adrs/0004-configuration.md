# ADR 0004: Configuración tipada por capas

- Estado: Accepted
- Fecha: 2026-08-06

## Decisión

YAML contiene defaults y seeds sin secretos; environment contiene secretos y
overrides de despliegue; DB contiene configuración funcional editable. El YAML
siembra por stable key únicamente si el registro no existe. Se rechazan claves
desconocidas y referencias inválidas al arrancar.

El colector HTTP genérico v1 admite GET JSON, allowlist de destino y JSONPath tipado
hacia resultados canónicos. No admite scripts, templates ejecutables ni acceso SQL.

## Consecuencias

Settings puede operar la plataforma sin redeploy y los despliegues siguen siendo
reproducibles. Cambiar YAML no sobrescribe silenciosamente decisiones del operador.

