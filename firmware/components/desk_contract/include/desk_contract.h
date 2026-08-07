#ifndef DESK_CONTRACT_H
#define DESK_CONTRACT_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#define DESK_SCHEMA_VERSION 1
#define DESK_DISPLAY_WIDTH 172
#define DESK_DISPLAY_HEIGHT 320
#define DESK_MAX_SCREENS 12
#define DESK_MAX_DASHBOARD_BYTES 65536
#define DESK_MAX_MQTT_BYTES 8192

typedef enum {
    DESK_SCREEN_UNKNOWN = 0,
    DESK_SCREEN_OVERVIEW,
    DESK_SCREEN_PRODUCT_STATUS,
    DESK_SCREEN_SERVICE_STATUS,
    DESK_SCREEN_ALERT,
    DESK_SCREEN_DEPLOYMENT,
    DESK_SCREEN_MARKET,
    DESK_SCREEN_WEATHER,
    DESK_SCREEN_METRIC_LIST,
    DESK_SCREEN_MESSAGE,
} desk_screen_type_t;

typedef enum {
    DESK_PRIORITY_CRITICAL = 0,
    DESK_PRIORITY_WARNING,
    DESK_PRIORITY_INFO,
    DESK_PRIORITY_SUCCESS,
} desk_priority_t;

typedef struct {
    char id[65];
    desk_screen_type_t type;
    int priority;
    uint32_t duration_ms;
    char title[49];
    char primary[81];
    char secondary[161];
    char status[24];
} desk_screen_t;

typedef struct {
    int schema_version;
    char revision[65];
    int64_t generated_at_epoch;
    int64_t expires_at_epoch;
    size_t screen_count;
    desk_screen_t screens[DESK_MAX_SCREENS];
} desk_dashboard_t;

typedef enum {
    DESK_PARSE_OK = 0,
    DESK_PARSE_INVALID_JSON,
    DESK_PARSE_UNSUPPORTED_VERSION,
    DESK_PARSE_INVALID_CONTRACT,
    DESK_PARSE_TOO_LARGE,
} desk_parse_result_t;

bool desk_contract_supports_version(int version);
desk_screen_type_t desk_contract_screen_type(const char *value);
const char *desk_contract_screen_name(desk_screen_type_t type);
desk_parse_result_t desk_contract_parse_dashboard(const char *json, size_t length,
                                                   desk_dashboard_t *dashboard);

#endif
