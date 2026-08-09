#ifndef DESK_CLOUD_H
#define DESK_CLOUD_H

#include <stddef.h>
#include <stdbool.h>
#include "desk_contract.h"

#define DESK_CLOUD_TOKEN_BUF_SIZE 512

/* Exchanges the device's provisioned secret (Kconfig) for a short-lived
 * bearer token via POST /device/auth/token. */
bool desk_cloud_authenticate(char *token_out, size_t token_out_size);

/* Fetches GET /device/dashboard with the given bearer token and parses it
 * straight through the shared contract parser. */
bool desk_cloud_fetch_dashboard(const char *access_token, desk_dashboard_t *out);

#endif
