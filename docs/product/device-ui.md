# Sistema visual del dispositivo

La pantalla contractual es ST7789 de 172×320 px en orientación vertical. El
dispositivo no decide contenido: recibe pantallas tipadas, aplica estos tokens y
mantiene el último snapshot válido. Los layouts son determinísticos y no realizan
scroll.

## Anatomía común

- Safe area de 8 px en los cuatro lados.
- Header de 34 px: icono semántico, título truncado a dos líneas y estado.
- Cuerpo flexible entre header y footer.
- Footer de 20 px: freshness/conectividad y progreso de rotación.
- Tipografía sans para texto y monoespaciada para valores numéricos.
- Todo texto que exceda el contrato se elide; nunca desplaza el footer.

Los tokens canónicos viven en
[`contracts/display/display-tokens.yaml`](../../contracts/display/display-tokens.yaml).
El simulador web usa los mismos valores y trabaja en un viewport CSS exacto de
172×320.

## Catálogo de pantallas

| Tipo | Jerarquía visual | Variantes obligatorias |
|---|---|---|
| `overview` | estado general, conteos y resumen | operational, degraded, outage |
| `product_status` | producto, ratio de servicios y detalle | cinco estados de servicio |
| `service_status` | servicio, estado, latencia y último cambio | latencia ausente, stale |
| `alert` | severidad dominante, título y mensaje | critical, warning, info, success |
| `deployment` | resultado, repo/branch, workflow, commit y autor | cinco estados de deploy |
| `market` | símbolo, cotización, variación, bid/ask | up, down, flat, stale |
| `weather` | temperatura, condición, mínima/máxima y lluvia | día, stale, dato parcial |
| `metric_list` | hasta cinco pares label/value | tendencias y colores opcionales |
| `message` | icono, título y texto centrado | cuatro estilos semánticos |

## Alertas e interrupción

Una alerta crítica se renderiza por encima de la rotación con borde y acento de
severidad. El progreso de la pantalla anterior se conserva. Al resolverse o vencer,
la rotación vuelve al índice y tiempo restante anteriores. Warning, info y success
usan el mismo componente sin parpadeos; el LED RGB sigue `style.led_rgb`.

## Estados de sistema

- `loading`: logo simple y progreso indeterminado, solo antes del primer snapshot.
- `offline`: conserva contenido, muestra badge offline y freshness creciente.
- `stale`: conserva contenido y usa acento amarillo; no reemplaza valores por cero.
- `invalid`: descarta el documento nuevo y conserva el último snapshot válido.
- `empty`: mensaje explícito “Sin pantallas configuradas”, nunca pantalla negra.

## Gate visual

Cada tipo debe renderizar usando los valores máximos permitidos por JSON Schema sin
overflow, mantener contraste WCAG AA cuando sea aplicable y ser legible a escala
física 1×. Los fixtures contractuales son la entrada compartida de simulador,
backend y pruebas host-side del firmware.
