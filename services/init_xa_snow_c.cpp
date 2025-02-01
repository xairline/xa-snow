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

#include <cstdio>
#include <string>

#include "XPLMUtilities.h"

#include "xa-snow_c-impl.h"
#include "airport.h"

std::string xp_dir;
XPLMDataRef plane_lat_dr, plane_lon_dr, plane_elevation_dr, plane_true_psi_dr,
	plane_y_agl_dr;

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

		CollectAirports(xp_dir);
		log_msg("InitXaSnowC done, xp_dir: '%s'", xp_dir.c_str());
	}
}

extern "C" float
LegacyAirportSnowDepth(float snow_depth)		// -> adjusted snow depth
{
	// nothing for now
	float lat = XPLMGetDataf(plane_lat_dr);
	float lon = XPLMGetDataf(plane_lon_dr);
	return snow_depth;
}
