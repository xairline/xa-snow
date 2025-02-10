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

#include <iostream>
#include <fstream>
#include <sstream>
#include <vector>
#include <string>
#include <stdexcept>
#include <cmath>

#include "xa-snow.h"

// depth map of the world in 0.1° resolution
static constexpr int n_iLon = 3600;
static constexpr int n_iLat = 1801;


class DepthMap {
    friend DepthMap* ElsaOnTheCoast(DepthMap* gribSnow);

protected:
    float val[n_iLon][n_iLat] = {0};

public:
    DepthMap() { log_msg("DepthMap created: %p", this); }
    ~DepthMap() { log_msg("DepthMap destoyed: %p", this); }
    float Get(float lon, float lat);
    float GetIdx(int iLon, int iLat);
    void LoadCsv(const char *csv_name);
};


float
DepthMap::GetIdx(int iLon, int iLat)
{
    // for lon we wrap around
    if (iLon >= n_iLon) {
        iLon -= n_iLon;
    } else if (iLon < 0) {
        iLon += n_iLon;
    }

    // for lat we just confine, doesn't make a difference anyway
    if (iLat >= n_iLat) {
        iLat = n_iLat - 1;
    } else if (iLat < 0) {
        iLat = 0;
    }

    return val[iLon][iLat];
}


float
DepthMap::Get(float lon, float lat)
{
    // our snow world map is 3600x1801 [0,359.9]x[0,180.0]
    lat += 90.0;

    // longitude is -180 to 180, we need to convert it to 0 to 360
    if (lon < 0) {
        lon += 360;
    }

    lon *= 10;
    lat *= 10;

    // index of tile is lower left corner
    int iLon = static_cast<int>(lon);
    int iLat = static_cast<int>(lat);

    // (s, t) coordinates of (lon, lat) within tile, s,t in [0,1]
    float s = lon - static_cast<float>(iLon);
    float t = lat - static_cast<float>(iLat);

    //m.Logger.Infof("(%f, %f) -> (%d, %d) (%f, %f)", lon/10, lat/10 - 90, iLon, iLat, s, t)
    float v00 = GetIdx(iLon, iLat);
    float v10 = GetIdx(iLon + 1, iLat);
    float v01 = GetIdx(iLon, iLat + 1);
    float v11 = GetIdx(iLon + 1, iLat + 1);

	// Lagrange polynoms: pij = is 1 on corner ij and 0 elsewhere
    float p00 = (1 - s) * (1 - t);
    float p10 = s * (1 - t);
    float p01 = (1 - s) * t;
    float p11 = s * t;

    float v = v00 * p00 + v10 * p10 + v01 * p01 + v11 * p11;
	//m.Logger.Infof("vij: %f, %f, %f, %f; v: %f", v00, v10, v01, v11, v)
    return v;
}

void
DepthMap::LoadCsv(const char *csv_name)
{
    std::ifstream file(csv_name);
    if (!file.is_open()) {
        log_msg("Error opening file: %s", csv_name);
        return;
    }

    std::string line;
    int counter = 0;

    // Skip the header
    std::getline(file, line);
    counter++;

    while (std::getline(file, line)) {
        std::istringstream ss(line);
        std::string token;
        std::vector<std::string> record;

        while (std::getline(ss, token, ',')) {
            record.push_back(token);
        }

        if (record.size() < 3) continue;

        float lon = std::stof(record[0]);
        float lat = std::stof(record[1]);
        float value = 0;

        if (record[2].find('e') != std::string::npos) {
            value = 0;
        } else {
            value = std::stof(record[2]);
        }

        // Convert longitude and latitude to array indices
        // This example assumes the CSV contains all longitudes and latitudes
        int x = static_cast<int>(lon * 10);         // Adjust these calculations based on your data's range and resolution
        int y = static_cast<int>((lat + 90) * 10);  // Adjust for negative latitudes

        val[x][y] = value;
        counter++;
    }

    log_msg("depth map size: %d",  counter);
    log_msg("Loading CSV file '%s': Done", csv_name);
}

DepthMap*
ElsaOnTheCoast(DepthMap* gribSnow)
{
    auto* new_dm = new DepthMap();

    const float min_sd = 0.02f; // only go higher than this snow depth
    int n_extend = 0;

    for (int i = 0; i < n_iLon; i++) {
        for (int j = 0; j < n_iLat; j++) {
            float sd = gribSnow->GetIdx(i, j);
            float sdn = new_dm->val[i][j]; // may already be set by inland extension earlier
            if (sd > sdn) { // always maximize
                new_dm->val[i][j] = sd;
            }

            const int max_step = 3; // to look for inland snow ~ 5 to 10 km / step
            bool is_coast;
            int dir_x, dir_y, dir_angle;
            std::tie(is_coast, dir_x, dir_y, dir_angle) = coast_map.is_coast(i, j);
            if (is_coast && sd <= min_sd) {
                // look for inland snow
                int inland_dist = 0;
                float inland_sd = 0.0f;
                for (int k = 1; k <= max_step; k++) {
                    int ii = i + k * dir_x;
                    int jj = j + k * dir_y;

                    if (k < max_step && coast_map.is_water(ii, jj)) { // if possible skip water
                        continue;
                    }

                    float tmp = gribSnow->GetIdx(ii, jj);
                    if (tmp > sd && tmp > min_sd) { // found snow
                        inland_dist = k;
                        inland_sd = tmp;
                        break;
                    }
                }

                const float decay = 0.8f; // snow depth decay per step
                if (inland_dist > 0) {
					//g.Logger.Infof("Inland snow detected for (%d, %d) at dist %d, sd: %0.3f %0.3f",
					//				 i, j, inland_dist, sd, inland_sd)

					// use exponential decay law from inland point to coast line point
                    for (int k = inland_dist - 1; k >= 0; k--) {
                        inland_sd *= decay;
                        if (inland_sd < min_sd) {
                            inland_sd = min_sd;
                        }
                        int x = i + k * dir_x;
                        int y = j + k * dir_y;
                        if (x >= n_iLon) {
                            x -= n_iLon;
                        }
                        if (x < 0) {
                            x += n_iLon;
                        }

                        // the poles are tricky so we just clamp
                        // anyway it does not make a difference
                        if (y >= n_iLat) {
                            y = n_iLat - 1;
                        }
                        if (y < 0) {
                            y = 0;
                        }
                        new_dm->val[x][y] = inland_sd;
                        n_extend++;
                    }
                }
            }
        }
    }

    log_msg("Extended coastal snow on %d grid points", n_extend);
    return new_dm;
}

// C++ to C translations that will eventually go away
#include "xa-snow-cgo.h"

extern "C"
uint64_t DMNewDepthMap()
{
    return (uint64_t)new DepthMap();
}

extern "C"
void DMLoadCsv(uint64_t ptr, char *fname)
{
    DepthMap* dm = reinterpret_cast<DepthMap*>(ptr);
    dm->LoadCsv(fname);
}

extern "C"
float DMGetIdx(uint64_t ptr, int iLon, int iLat)
{
    DepthMap* dm = reinterpret_cast<DepthMap*>(ptr);
    return dm->GetIdx(iLon, iLat);
}

extern "C"
float DMGet(uint64_t ptr, float lon, float lat)
{
    DepthMap* dm = reinterpret_cast<DepthMap*>(ptr);
    return dm->Get(lon, lat);
}

extern "C"
uint64_t DMElsaOnTheCoast(uint64_t ptr)
{
    DepthMap* dm = reinterpret_cast<DepthMap*>(ptr);
    return (uint64_t)ElsaOnTheCoast(dm);
}

extern "C"
void DMDestroyDepthMap(uint64_t ptr)
{
    DepthMap* dm = reinterpret_cast<DepthMap*>(ptr);
    delete(dm);
}
