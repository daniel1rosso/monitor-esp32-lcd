idf_component_register(
    SRCS "desk_display.c"
    INCLUDE_DIRS "include"
    REQUIRES desk_contract lvgl esp_driver_gpio esp_driver_spi esp_timer
)
