idf_component_register(
    SRCS "desk_cloud.c"
    INCLUDE_DIRS "include"
    REQUIRES desk_contract esp_http_client mbedtls espressif__cjson
)
