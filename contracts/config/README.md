# Contrato de configuración

`platform.example.yaml` muestra todos los orígenes de configuración sin contener
secretos reales. `${secret:NAME}` es una referencia: el loader la resuelve desde el
entorno o un secret file y falla de forma segura si falta.

El schema valida estructura y tipos básicos; la aplicación valida reglas cruzadas:
keys únicas, perfiles con pantallas soportadas, referencias de producto/servicio,
destinos HTTP incluidos en allowlist y secrets disponibles. La UI nunca recibe el
valor resuelto de una referencia secreta.

