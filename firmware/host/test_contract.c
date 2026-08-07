#include "desk_contract.h"
#include "desk_rotation.h"

#include <assert.h>

int main(void)
{
    assert(desk_contract_supports_version(1));
    assert(!desk_contract_supports_version(2));
    assert(desk_contract_screen_type("overview") == DESK_SCREEN_OVERVIEW);
    assert(desk_contract_screen_type("market") == DESK_SCREEN_MARKET);
    assert(desk_contract_screen_type("future_type") == DESK_SCREEN_UNKNOWN);
    assert(desk_contract_screen_type(0) == DESK_SCREEN_UNKNOWN);
	assert(desk_contract_screen_name(DESK_SCREEN_WEATHER)[0] == 'w');
    assert(DESK_MAX_SCREENS == 12);
    assert(DESK_DISPLAY_WIDTH == 172);
    assert(DESK_DISPLAY_HEIGHT == 320);
	desk_dashboard_t dashboard = {.schema_version=1,.screen_count=2};
	dashboard.screens[0]=(desk_screen_t){.type=DESK_SCREEN_OVERVIEW,.priority=50,.duration_ms=8000};
	dashboard.screens[1]=(desk_screen_t){.type=DESK_SCREEN_MARKET,.priority=20,.duration_ms=5000};
	desk_rotation_t rotation; desk_rotation_start(&rotation,&dashboard,100);
	assert(desk_rotation_current(&rotation)->type==DESK_SCREEN_OVERVIEW);
	assert(!desk_rotation_tick(&rotation,8099)); assert(desk_rotation_tick(&rotation,8100));
	assert(desk_rotation_current(&rotation)->type==DESK_SCREEN_MARKET);
	desk_screen_t alert={.type=DESK_SCREEN_ALERT,.priority=100,.duration_ms=3000};
	assert(desk_rotation_interrupt(&rotation,&alert,9000));assert(desk_rotation_current(&rotation)->type==DESK_SCREEN_ALERT);
	assert(desk_rotation_tick(&rotation,12000));assert(desk_rotation_current(&rotation)->type==DESK_SCREEN_MARKET);
    return 0;
}
