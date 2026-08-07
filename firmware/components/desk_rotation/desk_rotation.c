#include "desk_rotation.h"
#include <string.h>
void desk_rotation_start(desk_rotation_t *s,const desk_dashboard_t *d,uint64_t now){memset(s,0,sizeof(*s));s->dashboard=d;if(d&&d->screen_count)s->deadline_ms=now+d->screens[0].duration_ms;}
const desk_screen_t *desk_rotation_current(const desk_rotation_t *s){if(!s)return NULL;if(s->interrupted)return &s->alert;if(!s->dashboard||!s->dashboard->screen_count)return NULL;return &s->dashboard->screens[s->index];}
bool desk_rotation_tick(desk_rotation_t *s,uint64_t now){if(!s||now<s->deadline_ms)return false;if(s->interrupted){s->interrupted=false;const desk_screen_t *v=desk_rotation_current(s);if(v)s->deadline_ms=now+v->duration_ms;return true;}if(!s->dashboard||!s->dashboard->screen_count)return false;s->index=(s->index+1)%s->dashboard->screen_count;s->deadline_ms=now+s->dashboard->screens[s->index].duration_ms;return true;}
bool desk_rotation_interrupt(desk_rotation_t *s,const desk_screen_t *alert,uint64_t now){if(!s||!alert||alert->type!=DESK_SCREEN_ALERT||alert->priority<90)return false;s->alert=*alert;s->interrupted=true;s->deadline_ms=now+alert->duration_ms;return true;}
