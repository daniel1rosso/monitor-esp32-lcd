#ifndef DESK_RENDERER_H
#define DESK_RENDERER_H
#include <stdbool.h>
#include "desk_contract.h"
typedef struct { void *root; void *accent; void *title; void *primary; void *secondary; void *status; } desk_renderer_t;
bool desk_renderer_init(desk_renderer_t *renderer, void *parent);
void desk_renderer_render(desk_renderer_t *renderer, const desk_screen_t *screen);
#endif
