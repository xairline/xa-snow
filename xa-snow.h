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

#ifndef _XA_SNOW_H_
#define _XA_SNOW_H_

#include <string>
#include <tuple>
#include <numbers>

#define XPLM200
#define XPLM210
#define XPLM300
#define XPLM301

#include "XPLMDataAccess.h"
#include "XPLMScenery.h"

static constexpr float kD2R = std::numbers::pi/180.0;
static constexpr float kLat2m = 111120;                 // 1° lat in m
static constexpr float kF2M = 0.3048;                   // 1 ft [m]

extern XPLMDataRef plane_lat_dr, plane_lon_dr, plane_elevation_dr, plane_true_psi_dr,
	plane_y_agl_dr;

extern XPLMProbeInfo_t probeinfo;
extern XPLMProbeRef probe_ref;

extern std::string xp_dir;
extern std::string plugin_dir;
extern std::string output_dir;

// functions
extern void log_msg(const char *fmt, ...) __attribute__ ((format (printf, 1, 2)));
extern std::tuple<float, float, float> SnowDepthToXplaneSnowNow(float depth); // snowNow, snowAreaWidth, iceNow

struct CoastMap {
    // water map in 0.1° resolution
    static constexpr int n_wm = 3600;
    static constexpr int m_wm = 1800;

    uint8_t wmap [n_wm][m_wm];		// encoded as (dir << 2)|sXxx

    void wrap_ij(int i, int j, int &wrapped_i, int& wrapped_j);

  public:
    bool load(const std::string& dir);
    bool is_water(int i, int j);
    bool is_land(int i, int j);
    std::tuple<bool, int, int, int> is_coast(int i, int j); // -> yes_no, dir_x, dir_y, grid_angle
};

// depth map of the world in 0.1° resolution
static constexpr int n_iLon = 3600;
static constexpr int n_iLat = 1801;


class DepthMap {
    friend void ElsaOnTheCoast(const DepthMap& grib_snow, DepthMap& new_dm);

protected:
    float val[n_iLon][n_iLat] = {0};

public:
    DepthMap() { log_msg("DepthMap created: %p", this); }
    ~DepthMap() { log_msg("DepthMap destoyed: %p", this); }
    float Get(float lon, float lat) const;
    float GetIdx(int iLon, int iLat) const;
    void load_csv(const char *csv_name);
};

extern CoastMap coast_map;
extern DepthMap *grib_snod_map, *snod_map;

extern "C" void StartAsyncDownload(bool sys_time, int day, int month, int hour);
extern "C" bool CheckAsyncDownload();

extern int sub_exec(const std::string& command);
#endif
