#ifndef DESK_DISPLAY_H
#define DESK_DISPLAY_H

#include <stdint.h>

/* Brings up the onboard ST7789 panel (SPI + LVGL) and returns the active
 * lv_obj_t screen as an opaque pointer, ready to hand to desk_renderer_init.
 * Returns NULL if hardware init failed. */
void *desk_display_init(void);

/* Pumps LVGL's timer/animation/flush handling. Call periodically from the
 * main loop. */
void desk_display_handler(void);

/* Monotonic milliseconds, shared with LVGL's own tick source. */
uint64_t desk_display_now_ms(void);

#endif
