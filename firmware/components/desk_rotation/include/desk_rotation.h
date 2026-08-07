#ifndef DESK_ROTATION_H
#define DESK_ROTATION_H
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include "desk_contract.h"
typedef struct { const desk_dashboard_t *dashboard; size_t index; uint64_t deadline_ms; bool interrupted; desk_screen_t alert; } desk_rotation_t;
void desk_rotation_start(desk_rotation_t *state,const desk_dashboard_t *dashboard,uint64_t now_ms);
const desk_screen_t *desk_rotation_current(const desk_rotation_t *state);
bool desk_rotation_tick(desk_rotation_t *state,uint64_t now_ms);
bool desk_rotation_interrupt(desk_rotation_t *state,const desk_screen_t *alert,uint64_t now_ms);
#endif
