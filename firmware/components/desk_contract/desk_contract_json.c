#include "desk_contract.h"

#include <stdio.h>
#include <string.h>
#include "cJSON.h"

static bool copy_string(const cJSON *object, const char *key, char *target, size_t capacity, bool required)
{
    const cJSON *value = cJSON_GetObjectItemCaseSensitive(object, key);
    if (!cJSON_IsString(value)) return !required;
    size_t length = strlen(value->valuestring);
    if (length >= capacity) return false;
    memcpy(target, value->valuestring, length + 1);
    return true;
}

static void payload_text(const cJSON *payload, desk_screen_t *screen)
{
    const char *primary_keys[] = {"display_value", "name", "repository", "location", "body", "message"};
    const char *secondary_keys[] = {"detail", "message", "branch", "source"};
    const char *status_keys[] = {"status", "severity", "trend"};
    for (size_t i=0;i<sizeof(primary_keys)/sizeof(primary_keys[0]);i++) if(copy_string(payload,primary_keys[i],screen->primary,sizeof(screen->primary),false)&&screen->primary[0]) break;
    for (size_t i=0;i<sizeof(secondary_keys)/sizeof(secondary_keys[0]);i++) if(copy_string(payload,secondary_keys[i],screen->secondary,sizeof(screen->secondary),false)&&screen->secondary[0]) break;
    for (size_t i=0;i<sizeof(status_keys)/sizeof(status_keys[0]);i++) if(copy_string(payload,status_keys[i],screen->status,sizeof(screen->status),false)&&screen->status[0]) break;
    const cJSON *temperature=cJSON_GetObjectItemCaseSensitive(payload,"temperature_c");
    if(screen->primary[0]=='\0'&&cJSON_IsNumber(temperature)) snprintf(screen->primary,sizeof(screen->primary),"%.1f C",temperature->valuedouble);
    const cJSON *latency=cJSON_GetObjectItemCaseSensitive(payload,"latency_ms");
    if(screen->primary[0]=='\0'&&cJSON_IsNumber(latency)) snprintf(screen->primary,sizeof(screen->primary),"%d ms",latency->valueint);
}

desk_parse_result_t desk_contract_parse_dashboard(const char *json, size_t length, desk_dashboard_t *dashboard)
{
    if(json==NULL||dashboard==NULL||length==0) return DESK_PARSE_INVALID_CONTRACT;
    if(length>DESK_MAX_DASHBOARD_BYTES) return DESK_PARSE_TOO_LARGE;
    memset(dashboard,0,sizeof(*dashboard));
    cJSON *root=cJSON_ParseWithLength(json,length);
    if(root==NULL) return DESK_PARSE_INVALID_JSON;
    const cJSON *version=cJSON_GetObjectItemCaseSensitive(root,"schema_version");
    if(!cJSON_IsNumber(version)||!desk_contract_supports_version(version->valueint)){cJSON_Delete(root);return DESK_PARSE_UNSUPPORTED_VERSION;}
    const cJSON *screens=cJSON_GetObjectItemCaseSensitive(root,"screens");
    if(!copy_string(root,"revision",dashboard->revision,sizeof(dashboard->revision),true)||!cJSON_IsArray(screens)||cJSON_GetArraySize(screens)>DESK_MAX_SCREENS){cJSON_Delete(root);return DESK_PARSE_INVALID_CONTRACT;}
    dashboard->schema_version=version->valueint;
    cJSON *item=NULL;
    cJSON_ArrayForEach(item,screens){
        desk_screen_t *screen=&dashboard->screens[dashboard->screen_count];
        const cJSON *type=cJSON_GetObjectItemCaseSensitive(item,"type"),*priority=cJSON_GetObjectItemCaseSensitive(item,"priority"),*duration=cJSON_GetObjectItemCaseSensitive(item,"duration_ms"),*payload=cJSON_GetObjectItemCaseSensitive(item,"payload");
        if(!cJSON_IsString(type)||!cJSON_IsNumber(priority)||!cJSON_IsNumber(duration)||!cJSON_IsObject(payload)||!copy_string(item,"id",screen->id,sizeof(screen->id),true)||!copy_string(item,"title",screen->title,sizeof(screen->title),true)){cJSON_Delete(root);return DESK_PARSE_INVALID_CONTRACT;}
        screen->type=desk_contract_screen_type(type->valuestring); screen->priority=priority->valueint; screen->duration_ms=(uint32_t)duration->valuedouble;
        if(screen->type==DESK_SCREEN_UNKNOWN||screen->priority<0||screen->priority>100||screen->duration_ms<3000||screen->duration_ms>60000){cJSON_Delete(root);return DESK_PARSE_INVALID_CONTRACT;}
        payload_text(payload,screen); dashboard->screen_count++;
    }
    cJSON_Delete(root); return DESK_PARSE_OK;
}
