#include "desk_cloud.h"

#include <string.h>

#include "cJSON.h"
#include "esp_crt_bundle.h"
#include "esp_http_client.h"
#include "esp_log.h"

static const char *TAG = "desk_cloud";

#define DESK_CLOUD_AUTH_RESPONSE_BUF_SIZE 1024
#define DESK_CLOUD_DASHBOARD_RESPONSE_BUF_SIZE 16384
#define DESK_CLOUD_HTTP_TIMEOUT_MS 10000

typedef struct {
    char *buf;
    size_t cap;
    size_t len;
} http_body_t;

static esp_err_t http_event_handler(esp_http_client_event_t *evt)
{
    if (evt->event_id == HTTP_EVENT_ON_DATA) {
        http_body_t *body = (http_body_t *)evt->user_data;
        if (body && body->len + (size_t)evt->data_len < body->cap) {
            memcpy(body->buf + body->len, evt->data, evt->data_len);
            body->len += evt->data_len;
            body->buf[body->len] = '\0';
        }
    }
    return ESP_OK;
}

static bool http_request(const char *url, esp_http_client_method_t method, const char *bearer_token,
                          const char *post_body, http_body_t *body)
{
    body->buf[0] = '\0';
    body->len = 0;

    esp_http_client_config_t config = {
        .url = url,
        .method = method,
        .event_handler = http_event_handler,
        .user_data = body,
        .crt_bundle_attach = esp_crt_bundle_attach,
        .timeout_ms = DESK_CLOUD_HTTP_TIMEOUT_MS,
        .buffer_size = 2048,
    };
    esp_http_client_handle_t client = esp_http_client_init(&config);
    if (!client) {
        return false;
    }

    if (post_body) {
        esp_http_client_set_header(client, "Content-Type", "application/json");
        esp_http_client_set_post_field(client, post_body, (int)strlen(post_body));
    }
    if (bearer_token) {
        char auth_header[DESK_CLOUD_TOKEN_BUF_SIZE + 16];
        snprintf(auth_header, sizeof(auth_header), "Bearer %s", bearer_token);
        esp_http_client_set_header(client, "Authorization", auth_header);
    }

    esp_err_t err = esp_http_client_perform(client);
    int status = esp_http_client_get_status_code(client);
    esp_http_client_cleanup(client);

    if (err != ESP_OK) {
        ESP_LOGE(TAG, "request to %s failed: %s", url, esp_err_to_name(err));
        return false;
    }
    if (status != 200) {
        ESP_LOGE(TAG, "request to %s returned status %d", url, status);
        return false;
    }
    return true;
}

bool desk_cloud_authenticate(char *token_out, size_t token_out_size)
{
    static char response_buf[DESK_CLOUD_AUTH_RESPONSE_BUF_SIZE];
    http_body_t body = {.buf = response_buf, .cap = sizeof(response_buf), .len = 0};

    char request_body[256];
    snprintf(request_body, sizeof(request_body), "{\"device_id\":\"%s\",\"secret\":\"%s\",\"firmware_version\":\"%s\"}",
             CONFIG_DESK_DEVICE_ID, CONFIG_DESK_DEVICE_SECRET, "0.1.0");

    char url[256];
    snprintf(url, sizeof(url), "%s/device/auth/token", CONFIG_DESK_API_BASE_URL);

    if (!http_request(url, HTTP_METHOD_POST, NULL, request_body, &body)) {
        return false;
    }

    cJSON *root = cJSON_Parse(body.buf);
    if (!root) {
        ESP_LOGE(TAG, "auth response is not valid JSON");
        return false;
    }
    const cJSON *token = cJSON_GetObjectItemCaseSensitive(root, "access_token");
    bool ok = cJSON_IsString(token) && token->valuestring[0] && strlen(token->valuestring) < token_out_size;
    if (ok) {
        strcpy(token_out, token->valuestring);
    } else {
        ESP_LOGE(TAG, "auth response missing access_token");
    }
    cJSON_Delete(root);
    return ok;
}

bool desk_cloud_fetch_dashboard(const char *access_token, desk_dashboard_t *out)
{
    static char response_buf[DESK_CLOUD_DASHBOARD_RESPONSE_BUF_SIZE];
    http_body_t body = {.buf = response_buf, .cap = sizeof(response_buf), .len = 0};

    char url[256];
    snprintf(url, sizeof(url), "%s/device/dashboard", CONFIG_DESK_API_BASE_URL);

    if (!http_request(url, HTTP_METHOD_GET, access_token, NULL, &body)) {
        return false;
    }

    desk_parse_result_t result = desk_contract_parse_dashboard(body.buf, body.len, out);
    if (result != DESK_PARSE_OK) {
        ESP_LOGE(TAG, "dashboard parse failed result=%d", result);
        return false;
    }
    return true;
}
