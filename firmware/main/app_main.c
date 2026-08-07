#include "desk_contract.h"
#include "desk_rotation.h"
#include "esp_log.h"

static const char *TAG = "desk_monitor";

void app_main(void)
{
	desk_dashboard_t dashboard = {0};
	desk_rotation_t rotation;
	desk_rotation_start(&rotation, &dashboard, 0);
	ESP_LOGI(TAG, "Desk Monitor contractual runtime ready");
    ESP_LOGI(TAG, "contract=%d display=%dx%d", DESK_SCHEMA_VERSION,
             DESK_DISPLAY_WIDTH, DESK_DISPLAY_HEIGHT);
}
