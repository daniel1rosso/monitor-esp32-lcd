idf_component_register(
    SRCS "desk_contract.c" "desk_contract_json.c"
    INCLUDE_DIRS "include"
    REQUIRES espressif__cjson
)
