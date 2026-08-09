#ifndef DESK_WIFI_H
#define DESK_WIFI_H

#include <stdbool.h>
#include <stdint.h>

/* Initializes NVS + netif + the Wi-Fi station using the SSID/password from
 * Kconfig, and blocks until an IP is obtained or timeout_ms elapses. */
bool desk_wifi_connect(uint32_t timeout_ms);

bool desk_wifi_is_connected(void);

#endif
