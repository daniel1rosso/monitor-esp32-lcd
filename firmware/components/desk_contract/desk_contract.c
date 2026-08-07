#include "desk_contract.h"

#include <stddef.h>
#include <string.h>

typedef struct {
    const char *name;
    desk_screen_type_t type;
} screen_type_entry_t;

static const screen_type_entry_t SCREEN_TYPES[] = {
    {"overview", DESK_SCREEN_OVERVIEW},
    {"product_status", DESK_SCREEN_PRODUCT_STATUS},
    {"service_status", DESK_SCREEN_SERVICE_STATUS},
    {"alert", DESK_SCREEN_ALERT},
    {"deployment", DESK_SCREEN_DEPLOYMENT},
    {"market", DESK_SCREEN_MARKET},
    {"weather", DESK_SCREEN_WEATHER},
    {"metric_list", DESK_SCREEN_METRIC_LIST},
    {"message", DESK_SCREEN_MESSAGE},
};

bool desk_contract_supports_version(int version)
{
    return version == DESK_SCHEMA_VERSION;
}

desk_screen_type_t desk_contract_screen_type(const char *value)
{
    if (value == NULL) {
        return DESK_SCREEN_UNKNOWN;
    }

    for (size_t i = 0; i < sizeof(SCREEN_TYPES) / sizeof(SCREEN_TYPES[0]); ++i) {
        if (strcmp(value, SCREEN_TYPES[i].name) == 0) {
            return SCREEN_TYPES[i].type;
        }
    }

    return DESK_SCREEN_UNKNOWN;
}

const char *desk_contract_screen_name(desk_screen_type_t type)
{
    for (size_t i = 0; i < sizeof(SCREEN_TYPES) / sizeof(SCREEN_TYPES[0]); ++i) {
        if (SCREEN_TYPES[i].type == type) {
            return SCREEN_TYPES[i].name;
        }
    }
    return "unknown";
}
