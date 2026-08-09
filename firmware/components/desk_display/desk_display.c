#include "desk_display.h"
#include "desk_contract.h"

#include "driver/gpio.h"
#include "driver/spi_master.h"
#include "esp_heap_caps.h"
#include "esp_log.h"
#include "esp_timer.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"

#include "lvgl.h"
#include "drivers/display/st7789/lv_st7789.h"

/* Waveshare ESP32-C6-LCD-1.47: ST7789, 172x320, SPI.
 * Pinout and panel geometry confirmed against the vendor's own ESPHome
 * reference config (SPI mode 3, x-gap 34, BGR order, inverted colors). */
#define PIN_LCD_SCLK 7
#define PIN_LCD_MOSI 6
#define PIN_LCD_CS   14
#define PIN_LCD_DC   15
#define PIN_LCD_RST  21
#define PIN_LCD_BL   22

#define LCD_SPI_HOST SPI2_HOST
#define LCD_SPI_HZ   (20 * 1000 * 1000)
#define LCD_GAP_X    34
#define LCD_GAP_Y    0
#define LCD_BUF_ROWS 40

static const char *TAG = "desk_display";
static spi_device_handle_t s_spi;

static uint32_t tick_get_cb(void)
{
    return (uint32_t)(esp_timer_get_time() / 1000);
}

static void spi_send(const uint8_t *data, size_t len, bool is_cmd)
{
    if (len == 0) {
        return;
    }
    gpio_set_level(PIN_LCD_DC, is_cmd ? 0 : 1);
    spi_transaction_t t = {0};
    t.length = len * 8;
    t.tx_buffer = data;
    spi_device_polling_transmit(s_spi, &t);
}

static void lcd_send_cmd_cb(lv_display_t *disp, const uint8_t *cmd, size_t cmd_size, const uint8_t *param,
                             size_t param_size)
{
    (void)disp;
    spi_send(cmd, cmd_size, true);
    if (param_size) {
        spi_send(param, param_size, false);
    }
}

static void lcd_send_color_cb(lv_display_t *disp, const uint8_t *cmd, size_t cmd_size, uint8_t *param,
                               size_t param_size)
{
    spi_send(cmd, cmd_size, true);
    spi_send(param, param_size, false);
    lv_display_flush_ready(disp);
}

static void hw_reset(void)
{
    gpio_set_level(PIN_LCD_RST, 0);
    vTaskDelay(pdMS_TO_TICKS(10));
    gpio_set_level(PIN_LCD_RST, 1);
    vTaskDelay(pdMS_TO_TICKS(120));
}

void *desk_display_init(void)
{
    gpio_config_t io_conf = {
        .pin_bit_mask = (1ULL << PIN_LCD_DC) | (1ULL << PIN_LCD_RST) | (1ULL << PIN_LCD_BL),
        .mode = GPIO_MODE_OUTPUT,
    };
    gpio_config(&io_conf);
    gpio_set_level(PIN_LCD_BL, 0);
    gpio_set_level(PIN_LCD_RST, 1);

    spi_bus_config_t buscfg = {
        .sclk_io_num = PIN_LCD_SCLK,
        .mosi_io_num = PIN_LCD_MOSI,
        .miso_io_num = -1,
        .quadwp_io_num = -1,
        .quadhd_io_num = -1,
        .max_transfer_sz = DESK_DISPLAY_WIDTH * LCD_BUF_ROWS * 2,
    };
    if (spi_bus_initialize(LCD_SPI_HOST, &buscfg, SPI_DMA_CH_AUTO) != ESP_OK) {
        ESP_LOGE(TAG, "spi_bus_initialize failed");
        return NULL;
    }

    spi_device_interface_config_t devcfg = {
        .clock_speed_hz = LCD_SPI_HZ,
        .mode = 3,
        .spics_io_num = PIN_LCD_CS,
        .queue_size = 4,
    };
    if (spi_bus_add_device(LCD_SPI_HOST, &devcfg, &s_spi) != ESP_OK) {
        ESP_LOGE(TAG, "spi_bus_add_device failed");
        return NULL;
    }

    hw_reset();

    lv_init();
    lv_tick_set_cb(tick_get_cb);

    lv_display_t *disp =
        lv_st7789_create(DESK_DISPLAY_WIDTH, DESK_DISPLAY_HEIGHT, LV_LCD_FLAG_BGR, lcd_send_cmd_cb, lcd_send_color_cb);
    if (!disp) {
        ESP_LOGE(TAG, "lv_st7789_create failed");
        return NULL;
    }
    lv_display_set_color_format(disp, LV_COLOR_FORMAT_RGB565_SWAPPED);

    size_t buf_bytes = (size_t)DESK_DISPLAY_WIDTH * LCD_BUF_ROWS * 2;
    void *buf1 = heap_caps_malloc(buf_bytes, MALLOC_CAP_DMA | MALLOC_CAP_INTERNAL);
    if (!buf1) {
        ESP_LOGE(TAG, "display buffer allocation failed");
        return NULL;
    }
    lv_display_set_buffers(disp, buf1, NULL, buf_bytes, LV_DISPLAY_RENDER_MODE_PARTIAL);

    lv_st7789_set_gap(disp, LCD_GAP_X, LCD_GAP_Y);
    lv_st7789_set_invert(disp, true);

    gpio_set_level(PIN_LCD_BL, 1);

    return lv_display_get_screen_active(disp);
}

void desk_display_handler(void)
{
    lv_timer_handler();
}

uint64_t desk_display_now_ms(void)
{
    return (uint64_t)(esp_timer_get_time() / 1000);
}
