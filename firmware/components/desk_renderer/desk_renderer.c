#include "desk_renderer.h"
#include <string.h>
#include "lvgl.h"

static uint32_t accent_for(const desk_screen_t *screen){
    if(screen->type==DESK_SCREEN_ALERT||strcmp(screen->status,"outage")==0||strcmp(screen->status,"critical")==0)return 0xff453a;
    if(strcmp(screen->status,"degraded")==0||strcmp(screen->status,"warning")==0)return 0xffcc00;
    if(screen->type==DESK_SCREEN_MARKET||strcmp(screen->status,"operational")==0||strcmp(screen->status,"success")==0)return 0x30d158;
    if(screen->type==DESK_SCREEN_MESSAGE)return 0xbf5af2;
    return 0x0a84ff;
}
bool desk_renderer_init(desk_renderer_t *r,void *parent_ptr){
    if(!r||!parent_ptr)return false;memset(r,0,sizeof(*r));lv_obj_t *parent=parent_ptr;
    lv_obj_t *root=lv_obj_create(parent);lv_obj_remove_style_all(root);lv_obj_set_size(root,DESK_DISPLAY_WIDTH,DESK_DISPLAY_HEIGHT);lv_obj_set_style_bg_color(root,lv_color_hex(0x070a10),0);lv_obj_set_style_bg_opa(root,LV_OPA_COVER,0);lv_obj_set_style_pad_all(root,10,0);lv_obj_set_flex_flow(root,LV_FLEX_FLOW_COLUMN);
    lv_obj_t *accent=lv_obj_create(root);lv_obj_remove_style_all(accent);lv_obj_set_size(accent,30,3);lv_obj_set_style_radius(accent,LV_RADIUS_CIRCLE,0);
    lv_obj_t *title=lv_label_create(root);lv_obj_set_width(title,152);lv_label_set_long_mode(title,LV_LABEL_LONG_DOT);lv_obj_set_style_text_color(title,lv_color_hex(0xf4f7fb),0);
    lv_obj_t *primary=lv_label_create(root);lv_obj_set_width(primary,152);lv_label_set_long_mode(primary,LV_LABEL_LONG_WRAP);lv_obj_set_style_text_color(primary,lv_color_hex(0xf4f7fb),0);lv_obj_set_style_pad_top(primary,34,0);
    lv_obj_t *secondary=lv_label_create(root);lv_obj_set_width(secondary,152);lv_label_set_long_mode(secondary,LV_LABEL_LONG_WRAP);lv_obj_set_style_text_color(secondary,lv_color_hex(0x9eabbd),0);lv_obj_set_flex_grow(secondary,1);
    lv_obj_t *status=lv_label_create(root);lv_obj_set_width(status,152);lv_label_set_long_mode(status,LV_LABEL_LONG_DOT);lv_obj_set_style_text_color(status,lv_color_hex(0x64748b),0);
    r->root=root;r->accent=accent;r->title=title;r->primary=primary;r->secondary=secondary;r->status=status;return true;
}
void desk_renderer_render(desk_renderer_t *r,const desk_screen_t *s){
    if(!r||!r->root||!s)return;uint32_t accent=accent_for(s);lv_obj_set_style_bg_color(r->accent,lv_color_hex(accent),0);lv_obj_set_style_bg_opa(r->accent,LV_OPA_COVER,0);lv_label_set_text(r->title,s->title);lv_label_set_text(r->primary,s->primary[0]?s->primary:desk_contract_screen_name(s->type));lv_label_set_text(r->secondary,s->secondary);lv_label_set_text(r->status,s->status);lv_obj_set_style_text_color(r->status,lv_color_hex(accent),0);
}
