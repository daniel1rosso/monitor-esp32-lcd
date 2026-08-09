#include "desk_cloud.h"
#include "desk_contract.h"
#include "desk_display.h"
#include "desk_renderer.h"
#include "desk_rotation.h"
#include "desk_wifi.h"
#include "esp_log.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include <stdio.h>

static const char *TAG = "desk_monitor";

/* Embedded verbatim from the shared contract fixture used by the backend,
 * the simulator and the host-side tests: the offline fallback shown until
 * (or unless) the cloud dashboard can be fetched. */
extern const uint8_t dashboard_json_start[] asm("_binary_dashboard_json_start");
extern const uint8_t dashboard_json_end[] asm("_binary_dashboard_json_end");

#define WIFI_CONNECT_TIMEOUT_MS 20000
#define TOKEN_REFRESH_INTERVAL_MS (600 * 1000)
#define DASHBOARD_REFRESH_INTERVAL_MS (60 * 1000)
#define CONNECTIVITY_REFRESH_INTERVAL_MS (5 * 1000)

static void update_connectivity_label(desk_renderer_t *renderer)
{
	char text[64];
	if (desk_wifi_is_connected()) {
		snprintf(text, sizeof(text), "Wi-Fi: %s", CONFIG_DESK_WIFI_SSID);
	} else {
		snprintf(text, sizeof(text), "Wi-Fi: sin conexion");
	}
	desk_renderer_set_connectivity(renderer, text);
}

void app_main(void)
{
	static desk_dashboard_t dashboard;
	desk_rotation_t rotation;
	desk_renderer_t renderer;
	static char access_token[DESK_CLOUD_TOKEN_BUF_SIZE];
	static desk_dashboard_t fetched;
	bool cloud_ready = false;

	size_t json_len = (size_t)(dashboard_json_end - dashboard_json_start);
	desk_parse_result_t parse_result =
		desk_contract_parse_dashboard((const char *)dashboard_json_start, json_len, &dashboard);
	ESP_LOGI(TAG, "contract=%d display=%dx%d parse_result=%d screens=%d", DESK_SCHEMA_VERSION,
		 DESK_DISPLAY_WIDTH, DESK_DISPLAY_HEIGHT, parse_result, (int)dashboard.screen_count);

	void *screen = desk_display_init();
	if (!screen) {
		ESP_LOGE(TAG, "display init failed");
		return;
	}
	desk_renderer_init(&renderer, screen);

	uint64_t now_ms = desk_display_now_ms();
	desk_rotation_start(&rotation, &dashboard, now_ms);
	desk_renderer_render(&renderer, desk_rotation_current(&rotation));

	ESP_LOGI(TAG, "Desk Monitor contractual runtime ready");

	update_connectivity_label(&renderer);

	if (desk_wifi_connect(WIFI_CONNECT_TIMEOUT_MS)) {
		ESP_LOGI(TAG, "wifi connected");
		update_connectivity_label(&renderer);
		if (desk_cloud_authenticate(access_token, sizeof(access_token))) {
			ESP_LOGI(TAG, "device authenticated");
			if (desk_cloud_fetch_dashboard(access_token, &fetched)) {
				dashboard = fetched;
				desk_rotation_start(&rotation, &dashboard, desk_display_now_ms());
				desk_renderer_render(&renderer, desk_rotation_current(&rotation));
				cloud_ready = true;
				ESP_LOGI(TAG, "cloud dashboard loaded screens=%d", (int)dashboard.screen_count);
			} else {
				ESP_LOGW(TAG, "cloud dashboard fetch failed, keeping fixture");
			}
		} else {
			ESP_LOGW(TAG, "device authentication failed, keeping fixture");
		}
	} else {
		ESP_LOGW(TAG, "wifi connect timed out, keeping fixture");
	}

	uint64_t last_token_refresh_ms = desk_display_now_ms();
	uint64_t last_dashboard_refresh_ms = desk_display_now_ms();
	uint64_t last_connectivity_refresh_ms = desk_display_now_ms();

	while (1) {
		desk_display_handler();
		now_ms = desk_display_now_ms();
		if (desk_rotation_tick(&rotation, now_ms)) {
			desk_renderer_render(&renderer, desk_rotation_current(&rotation));
		}

		if (now_ms - last_connectivity_refresh_ms >= CONNECTIVITY_REFRESH_INTERVAL_MS) {
			last_connectivity_refresh_ms = now_ms;
			update_connectivity_label(&renderer);
		}

		if (cloud_ready && desk_wifi_is_connected()) {
			if (now_ms - last_token_refresh_ms >= TOKEN_REFRESH_INTERVAL_MS) {
				last_token_refresh_ms = now_ms;
				if (desk_cloud_authenticate(access_token, sizeof(access_token))) {
					ESP_LOGI(TAG, "token refreshed");
				} else {
					ESP_LOGW(TAG, "token refresh failed");
				}
			}
			if (now_ms - last_dashboard_refresh_ms >= DASHBOARD_REFRESH_INTERVAL_MS) {
				last_dashboard_refresh_ms = now_ms;
				if (desk_cloud_fetch_dashboard(access_token, &fetched)) {
					dashboard = fetched;
					desk_rotation_start(&rotation, &dashboard, now_ms);
					desk_renderer_render(&renderer, desk_rotation_current(&rotation));
				} else {
					ESP_LOGW(TAG, "dashboard refresh failed");
				}
			}
		}

		vTaskDelay(pdMS_TO_TICKS(20));
	}
}
