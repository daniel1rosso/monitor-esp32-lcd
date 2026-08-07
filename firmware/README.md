# Desk Monitor Firmware

Runtime contractual ESP-IDF para Waveshare ESP32-C6-LCD-1.47, ST7789, 172×320.
La versión fijada para CI es ESP-IDF 6.0.2 y LVGL 9 se obtiene mediante el component
manager. Incluye parser defensivo, modelo de snapshot, rotación, interrupción de
alertas críticas y un renderer LVGL independiente del driver físico.

`desk_contract` valida y normaliza el JSON; `desk_rotation` decide qué pantalla se
muestra y cómo reanudarla; `desk_renderer` es el único componente que conoce LVGL.
El BSP, Wi-Fi, cliente HTTP/MQTT y almacenamiento NVS se conectan después sin
introducir dependencias en esos tres componentes.

OTA está explícitamente fuera de v1.

## Build ESP32-C6

```bash
idf.py set-target esp32c6
idf.py build
```

## Pruebas host-side

```bash
make -C host test
```

También se incluye un `CMakeLists.txt` host-side para entornos que usen CMake.
