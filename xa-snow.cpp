//
//    X Airline Snow: show accumulated snow in X-Plane's world
//
//    Copyright (C) 2025  Zodiac1214
//    Copyright (C) 2025  Holger Teutsch
//
//    This library is free software; you can redistribute it and/or
//    modify it under the terms of the GNU Lesser General Public
//    License as published by the Free Software Foundation; either
//    version 2.1 of the License, or (at your option) any later version.
//
//    This library is distributed in the hope that it will be useful,
//    but WITHOUT ANY WARRANTY; without even the implied warranty of
//    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
//    Lesser General Public License for more details.
//
//    You should have received a copy of the GNU Lesser General Public
//    License along with this library; if not, write to the Free Software
//    Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301
//    USA
//

// Coding style is loosely following Google guide: https://google.github.io/styleguide/cppguide.html

#include <cstring>
#include <cstdio>
#include <string>
#include <fstream>
#include <filesystem>

#include "xa-snow.h"

#include "XPLMPlugin.h"
#include "XPLMProcessing.h"
#include "XPLMUtilities.h"
#include "XPLMMenus.h"

#include "airport.h"
#include "depth_map.h"

std::string xp_dir, plugin_dir, output_dir, pref_path;

XPLMDataRef plane_lat_dr, plane_lon_dr, plane_elevation_dr, plane_true_psi_dr,
	plane_y_agl_dr, weather_mode_dr, rwy_cond_dr, sys_time_dr,
    sim_current_month_dr, sim_current_day_dr, sim_local_hours_dr,
    snow_dr, ice_dr, rwy_snow_dr;

XPLMProbeInfo_t probeinfo;
XPLMProbeRef probe_ref;
static XPLMMenuID xas_menu;

// preferences
static int pref_override, pref_rwy_ice, pref_historical, pref_autoupdate, pref_limit_snow;
// associated menu ids
static int override_item, rwy_ice_item, historical_item, autoupdate_item, limit_snow_item;

static int loop_cnt;

// Initialize static member variables
static const std::vector<float> snowDepthTab     = {0.01f, 0.02f, 0.03f, 0.05f, 0.10f, 0.20f, 0.25f};
static const std::vector<float> snowNowTab       = {0.90f, 0.70f, 0.60f, 0.30f, 0.15f, 0.06f, 0.05f};
static const std::vector<float> snowAreaWidthTab = {0.25f, 0.25f, 0.25f, 0.25f, 0.25f, 0.29f, 0.33f};
static const std::vector<float> iceNowTab        = {2.00f, 2.00f, 2.00f, 2.00f, 0.80f, 0.37f, 0.37f};

std::tuple<float, float, float>
SnowDepthToXplaneSnowNow(float depth) // snowNow, snowAreaWidth, iceNow
{
    if (depth >= snowDepthTab.back()) {
        return std::make_tuple(snowNowTab.back(), snowAreaWidthTab.back(), iceNowTab.back());
    }

    if (depth <= snowDepthTab.front()) {
        return std::make_tuple(1.2f, snowAreaWidthTab.front(), iceNowTab.front());
    }

    // piecewise linear interpolation
    float snowNowValue = 1.2f;
    float iceNowValue = iceNowTab.front();
    float snowAreaWidthValue = snowAreaWidthTab.front();

    for (size_t i = 0; i < snowDepthTab.size() - 1; ++i) {
        float sd0 = snowDepthTab[i];
        float sd1 = snowDepthTab[i + 1];
        if (sd0 <= depth && depth < sd1) {
            float x = (depth - sd0) / (sd1 - sd0);
            snowNowValue = snowNowTab[i] + x * (snowNowTab[i + 1] - snowNowTab[i]);
            snowAreaWidthValue = snowAreaWidthTab[i] + x * (snowAreaWidthTab[i + 1] - snowAreaWidthTab[i]);
            iceNowValue = iceNowTab[i] + x * (iceNowTab[i + 1] - iceNowTab[i]);
            break;
        }
    }

    return std::make_tuple(snowNowValue, snowAreaWidthValue, iceNowValue);
}

static void
SavePrefs()
{
    FILE *f = fopen(pref_path.c_str(), "w");
    if (NULL == f)
        return;

    fprintf(f, "%d,%d,%d,%d,%d", pref_override, pref_rwy_ice, pref_historical, pref_autoupdate, pref_limit_snow);
    fclose(f);

    log_msg("Saving preferences to '%s'",  pref_path.c_str());
    log_msg("pref_override: %d, pref_rwy_ice: %d, pref_historical: %d, pref_autoupdate: %d, pref_limit_snow: %d",
             pref_override, pref_rwy_ice, pref_historical, pref_autoupdate, pref_limit_snow);
}

static void
LoadPrefs()
{
    pref_override = pref_rwy_ice = pref_historical = pref_autoupdate = pref_limit_snow = false;

    FILE *f  = fopen(pref_path.c_str(), "r");
    if (NULL == f)
        return;

    log_msg("Loading preferences from '%s'",  pref_path.c_str());

    [[maybe_unused]]int n = fscanf(f, "%i,%i,%i,%i,%i",
                                   &pref_override, &pref_rwy_ice, &pref_historical, &pref_autoupdate, &pref_limit_snow);
    fclose(f);

    log_msg("pref_override: %d, pref_rwy_ice: %d, pref_historical: %d, pref_autoupdate: %d, pref_limit_snow: %d",
             pref_override, pref_rwy_ice, pref_historical, pref_autoupdate, pref_limit_snow);
}

static void
MenuCB([[maybe_unused]] void *menu_ref, void *item_ref)
{
    int *pref = (int *)item_ref;

    int item;
    if (pref == &pref_override) {
        item = override_item;
    } else if (pref == &pref_rwy_ice) {
        item = rwy_ice_item;
    } else if (pref == &pref_historical) {
        item = historical_item;
        loop_cnt = 0;   // reload snow
    } else if (pref == &pref_autoupdate) {
        item = autoupdate_item;
    } else if (pref == &pref_limit_snow) {
        item = limit_snow_item;
    } else
        return;

    *pref = ! *pref;
    XPLMCheckMenuItem(xas_menu, item, *pref ? xplm_Menu_Checked : xplm_Menu_Unchecked);
}

// private drefs need delayed initialization
static bool
InitPrivateDrefs()
{
    static bool drefs_inited;

	if (! drefs_inited) {
        drefs_inited = true;
		bool success = true;
		snow_dr = XPLMFindDataRef("sim/private/controls/wxr/snow_now");
		success = success && (snow_dr != NULL);

		ice_dr = XPLMFindDataRef("sim/private/controls/wxr/ice_now");
		success = success && (ice_dr != NULL);

		rwy_snow_dr = XPLMFindDataRef("sim/private/controls/twxr/snow_area_width");
		success = success && (rwy_snow_dr != NULL);

		if (!success) {
			log_msg("Could not map required private datarefs");
			return false;
		}
	}

	return true;
}

static float
FlightLoopCb([[maybe_unused]] float inElapsedSinceLastCall,
             [[maybe_unused]] float inElapsedTimeSinceLastFlightLoop, [[maybe_unused]] int inCounter,
             [[maybe_unused]] void *inRefcon)
{
    static float snow_depth, snow_now, rwy_snow, ice_now;

    if (loop_cnt == 0) {
        loop_cnt++;
        log_msg("Flightloop (re)starting, kicking off");

        if (!InitPrivateDrefs())
            return 0; // Bye, if we don't have them by now we will never get them

        if (!pref_historical)
            StartAsyncDownload(true, 0, 0, 0);
        else {
            bool sys_time = (XPLMGetDatai(sys_time_dr) == 1);
            int day = XPLMGetDatai(sim_current_day_dr);
            int month = XPLMGetDatai(sim_current_month_dr);
            int hour = XPLMGetDatai(sim_local_hours_dr);
            StartAsyncDownload(sys_time, month, day, hour);
        }

        // set to known "no snow" values
        snow_depth = 0.0f;
        std::tie(snow_now, rwy_snow, ice_now) = SnowDepthToXplaneSnowNow(snow_depth);
        return 10.0f;
    }

    CheckAsyncDownload();

    // if manual weather and not override do nothing
    if ((1 != XPLMGetDatai(weather_mode_dr)) && !pref_override)
        return 5.0f;

    loop_cnt++;
    if (snod_map == nullptr) {
        log_msg("... waiting for snow map");
        return 5.0f;
    }

    // throttle computations
    if (loop_cnt % 8 == 0) {
        float snow_depth_n = snod_map->get(XPLMGetDataf(plane_lon_dr), XPLMGetDataf(plane_lat_dr));
        if (pref_limit_snow) {
            snow_depth_n = LegacyAirportSnowDepth(snow_depth_n);
        }

        const float alpha = 0.7f;
        snow_depth = alpha * snow_depth_n + (1 - alpha) * snow_depth;

        // If we have no accumulated snow leave the datarefs alone and
        // let X-Plane do its weather effect things
        if ((snow_depth < 0.001f) && pref_override)
            return -1;

        std::tie(snow_now, rwy_snow, ice_now) = SnowDepthToXplaneSnowNow(snow_depth);
    }

    // If we have no accumulated snow leave the datarefs alone and
    // let X-Plane do its weather effect things
    if ((snow_depth < 0.001f) && !pref_override)
        return -1;

    if (!pref_rwy_ice) {
        ice_now = 2;
        rwy_snow = 0;
        XPLMSetDataf(rwy_cond_dr, 0.0f);
    }

    XPLMSetDataf(snow_dr, snow_now);
    XPLMSetDataf(rwy_snow_dr, rwy_snow);
    XPLMSetDataf(ice_dr, ice_now);
    float rwy_cond = XPLMGetDataf(rwy_cond_dr);
    if (rwy_cond >= 4.0f) {
        rwy_cond = rwy_cond / 3.0f;
        XPLMSetDataf(rwy_cond_dr, rwy_cond);
    }

    return -1;
}


// =========================== plugin entry points ===============================================
PLUGIN_API int
XPluginStart(char *out_name, char *out_sig, char *out_desc)
{
    log_msg("Startup " VERSION);

    strcpy(out_name, "X Airline Snow - " VERSION);
    strcpy(out_sig, "com.github.xairline.xa-snow");
    strcpy(out_desc, "show accumulated snow in X-Plane's world");

    // Always use Unix-native paths on the Mac!
    XPLMEnableFeature("XPLM_USE_NATIVE_PATHS", 1);
    XPLMEnableFeature("XPLM_USE_NATIVE_WIDGET_WINDOWS", 1);

    char buffer[2048];
    XPLMGetSystemPath(buffer);
    xp_dir = std::string(buffer);   // has trailing slash
    plugin_dir = xp_dir + "Resources/plugins/xa-snow";
    output_dir = xp_dir + "Output/snow";
	pref_path = xp_dir + "Output/preferences/xa-snow.prf";
    std::filesystem::create_directory(output_dir);

    LoadPrefs();

    // map std API datarefs
    plane_lat_dr = XPLMFindDataRef("sim/flightmodel/position/latitude");
    plane_lon_dr = XPLMFindDataRef("sim/flightmodel/position/longitude");
    plane_elevation_dr = XPLMFindDataRef("sim/flightmodel/position/elevation");
    plane_true_psi_dr = XPLMFindDataRef("sim/flightmodel2/position/true_psi");
    plane_y_agl_dr = XPLMFindDataRef("sim/flightmodel2/position/y_agl");

    weather_mode_dr = XPLMFindDataRef("sim/weather/region/weather_source");
	rwy_cond_dr = XPLMFindDataRef("sim/weather/region/runway_friction");

    sys_time_dr = XPLMFindDataRef("sim/time/use_system_time");
    sim_current_month_dr = XPLMFindDataRef("sim/cockpit2/clock_timer/current_month");
    sim_current_day_dr = XPLMFindDataRef("sim/cockpit2/clock_timer/current_day");
    sim_local_hours_dr = XPLMFindDataRef("sim/cockpit2/clock_timer/local_time_hours");

    probeinfo.structSize = sizeof(XPLMProbeInfo_t);
    probe_ref = XPLMCreateProbe(xplm_ProbeY);

    CollectAirports(xp_dir);

    coast_map.load(plugin_dir);

    // build menues
    XPLMMenuID menu = XPLMFindPluginsMenu();
    xas_menu = XPLMCreateMenu("X Airline Snow", menu,
                              XPLMAppendMenuItem(menu, "X Airline Snow", NULL, 0),
                              MenuCB, NULL);

	override_item = XPLMAppendMenuItem(xas_menu, "Toggle Override", &pref_override, 0);
	rwy_ice_item = XPLMAppendMenuItem(xas_menu, "Lock Elsa up (ice)", &pref_rwy_ice, 0);
	historical_item = XPLMAppendMenuItem(xas_menu, "Enable Historical Snow", &pref_historical, 0);
	autoupdate_item = XPLMAppendMenuItem(xas_menu, "Enable Snow Depth Auto Update", &pref_autoupdate, 0);
    limit_snow_item = XPLMAppendMenuItem(xas_menu, "Limit snow for legacy airports", &pref_limit_snow, 0);

    XPLMCheckMenuItem(xas_menu, override_item, pref_override ? xplm_Menu_Checked : xplm_Menu_Unchecked);
    XPLMCheckMenuItem(xas_menu, rwy_ice_item, pref_rwy_ice ? xplm_Menu_Checked : xplm_Menu_Unchecked);
    XPLMCheckMenuItem(xas_menu, historical_item, pref_historical ? xplm_Menu_Checked : xplm_Menu_Unchecked);
    XPLMCheckMenuItem(xas_menu, autoupdate_item, pref_autoupdate ? xplm_Menu_Checked : xplm_Menu_Unchecked);
    XPLMCheckMenuItem(xas_menu, limit_snow_item, pref_limit_snow ? xplm_Menu_Checked : xplm_Menu_Unchecked);

    log_msg("XPluginStart done, xp_dir: '%s'", xp_dir.c_str());

    // ... and off we go
    XPLMRegisterFlightLoopCallback(FlightLoopCb, 2.0, NULL);
    return 1;
}

PLUGIN_API void
XPluginStop(void)
{
}

PLUGIN_API int
XPluginEnable(void)
{
    loop_cnt = 0;   // reinit snow download
    return 1;
}

PLUGIN_API void
XPluginDisable(void)
{
    SavePrefs();
    snod_map = nullptr;
}

PLUGIN_API void
XPluginReceiveMessage([[maybe_unused]] XPLMPluginID in_from, long in_msg, void *in_param)
{
    if (((in_msg == XPLM_MSG_PLANE_LOADED && in_param == 0) || (in_msg == XPLM_MSG_SCENERY_LOADED))
        && pref_autoupdate) {
        log_msg("Plane/Scenery loaded, reloading snow");
        loop_cnt = 0;
    }
}
