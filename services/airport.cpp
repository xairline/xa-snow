//
//    A contribution to https://github.com/xairline/xa-snow
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

#include "airport.h"
#include "XPLMGraphics.h"

static float constexpr kArptLimit = 18000;  // m, 10 nm
static float constexpr kArptSnow = 0.07;    // m snow

static float constexpr kMecSlope = 5;       // ° slope towards MEC
static float constexpr kMecOfs = 100.0;     // m vertical

extern "C" float
LegacyAirportSnowDepth(float snow_depth)		// -> adjusted snow depth
{
    if (snow_depth < kArptSnow)
        return snow_depth;

    float snow_depth_in = snow_depth;

    // look whether we are approaching a legacy airport
    LLPos pos = { XPLMGetDataf(plane_lon_dr), XPLMGetDataf(plane_lat_dr) };

    for (auto & arpt : airports) {
        float dist = len(pos - arpt->mec_center);
        if (dist < kArptLimit) {
            if (arpt->elevation == Airport::kNoElevation) {
                double x, y, z;
                const LLPos& pos = arpt->runways[0].end1;
                XPLMWorldToLocal(pos.lat, pos.lon, 0, &x, &y, &z);
                if (xplm_ProbeHitTerrain != XPLMProbeTerrainXYZ(probe_ref, x, y, z, &probeinfo)) {
                    log_msg("terrain probe failed???");
                }

                double dummy, elev;
                XPLMLocalToWorld(probeinfo.locationX, probeinfo.locationY, probeinfo.locationZ,
                                 &dummy, &dummy, &elev);
                arpt->elevation = elev;
                log_msg("elevation of '%s', %0.1f ft", arpt->name.c_str(), arpt->elevation / kF2M);
            }

            float haa = XPLMGetDataf(plane_elevation_dr) - arpt->elevation;
            float ref_haa = dist * 0.087f;     // a 5° slope into the center
            float dh = std::max(0.0f, haa - ref_haa);   // delta above ref slope
            float ref_dist = dist + 10.0f * dh;                          // is weighted higher
            float a = (ref_dist - arpt->mec_radius) / (kArptLimit - arpt->mec_radius);
            a = clampf(a, 0.0f, 1.0f);
            snow_depth = kArptSnow + a * (std::min(snow_depth, 0.25f) - kArptSnow);

            log_msg("haa: %.0f, ref_haa: %0.f, dist to '%s', %.0f m, snow_depth in: %0.2f, out: %0.2f",
                    haa, ref_haa, arpt->name.c_str(), dist, snow_depth_in, snow_depth);
            break;
        }
    }

	return snow_depth;
}
