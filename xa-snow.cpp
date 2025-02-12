//
//    A contribution to https://github.com/xairline/xa-snow by zodiac1214
//
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

#include <cstdio>
#include <string>
#include <filesystem>

#include "XPLMUtilities.h"

#include "xa-snow.h"
#include "airport.h"

std::string xp_dir, plugin_dir, output_dir;

XPLMDataRef plane_lat_dr, plane_lon_dr, plane_elevation_dr, plane_true_psi_dr,
	plane_y_agl_dr;

XPLMProbeInfo_t probeinfo;
XPLMProbeRef probe_ref;

DepthMap *grib_snod_map, *snod_map;

extern "C" void
InitXaSnowC()
{
	static bool init_done;
	if (! init_done) {
		init_done = true;

		plane_lat_dr = XPLMFindDataRef("sim/flightmodel/position/latitude");
		plane_lon_dr = XPLMFindDataRef("sim/flightmodel/position/longitude");
		plane_elevation_dr = XPLMFindDataRef("sim/flightmodel/position/elevation");
		plane_true_psi_dr = XPLMFindDataRef("sim/flightmodel2/position/true_psi");
		plane_y_agl_dr = XPLMFindDataRef("sim/flightmodel2/position/y_agl");

		char buffer[2048];
		XPLMGetSystemPath(buffer);
		xp_dir = std::string(buffer);
        plugin_dir = xp_dir + "/Resources/plugins/xa-snow";
        output_dir = xp_dir + "/Output/snow";
        std::filesystem::create_directory(output_dir);

        probeinfo.structSize = sizeof(XPLMProbeInfo_t);
        probe_ref = XPLMCreateProbe(xplm_ProbeY);

		CollectAirports(xp_dir);

        coast_map.load(plugin_dir);
        grib_snod_map = new DepthMap();
        snod_map = new DepthMap();

		log_msg("InitXaSnowC done, xp_dir: '%s'", xp_dir.c_str());
	}
}

extern "C" float
GetSnowDepth(float lat, float lon)
{
    return snod_map->Get(lon, lat);
}

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

// C++ to C translations that will eventually go away
#include "xa-snow-cgo.h"

extern "C"
R_SnowDepthToXplaneSnowNow CSnowDepthToXplaneSnowNow(float depth)
{
    R_SnowDepthToXplaneSnowNow r;
    std::tie(r.snowNow, r.snowAreaWidth, r.iceNow) = SnowDepthToXplaneSnowNow(depth);
    return r;
}
