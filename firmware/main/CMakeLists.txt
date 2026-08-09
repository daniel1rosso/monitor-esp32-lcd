idf_component_register(SRCS "app_main.c" INCLUDE_DIRS "."
    REQUIRES desk_contract desk_rotation desk_renderer desk_display desk_wifi desk_cloud
    EMBED_TXTFILES "../../contracts/json-schema/fixtures/dashboard.json")
