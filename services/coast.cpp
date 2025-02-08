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
#include <memory>
#include <cmath>
#include <string>
#include <png.h> // For image processing

#include "xa-snow.h"

// water map in 0.1° resolution
static constexpr int n_wm = 3600;
static constexpr int m_wm = 1800;

// we use a "grid direction" = 360°/45° in standard math convention
// 0 -> x, 2 -> y, 4 -> -x, ...
static const int dir_x[8] = {1, 1, 0, -1, -1, -1, 0, 1};
static const int dir_y[8] = {0, 1, 1, 1, 0, -1, -1, -1};

enum State {
    sWater,
    sLand,
    sCoast
};

struct CoastMap {
    uint8_t wmap [n_wm][m_wm];		// encoded as (dir << 2)|sXxx

    void wrap_ij(int i, int j, int &wrapped_i, int& wrapped_j) {
        if (i >= n_wm) {
            i -= n_wm;
        } else if (i < 0) {
            i += n_wm;
        }

        if (j >= m_wm) {
            j = m_wm - 1;
        } else if (j < 0) {
            j = 0;
        }

        wrapped_i =i; wrapped_j = j;
        return;
    }

  public:
    bool load(const std::string& dir);
    bool is_water(int i, int j);
    bool is_land(int i, int j);
    R_IsCoast is_coast(int i, int j); // -> yes_no, dir_x, dir_y, grid angle
};

static CoastMap coast_map;

bool
CoastMap::is_water(int i, int j)
{
    int wrapped_i, wrapped_j;
    wrap_ij(i, j, wrapped_i, wrapped_j);
    return (wmap[wrapped_i][wrapped_j] & 0x3) == sWater;
}

bool
CoastMap::is_land(int i, int j)
{
    int wrapped_i, wrapped_j;
    wrap_ij(i, j, wrapped_i, wrapped_j);
    return (wmap[wrapped_i][wrapped_j] & 0x3) == sLand;
}

R_IsCoast
CoastMap::is_coast(int i, int j)
{
    if (j >= m_wm) {
        return {false, 0, 0, 0};
    }

    uint8_t v = wmap[i][j];
    bool yes_no = (v & 0x3) == sCoast;
    int dir = v >> 2;
    return {yes_no, dir_x[dir], dir_y[dir], dir};
}

bool
CoastMap::load(const std::string& dir)
{
    std::string filename = dir + "/ESACCI-LC-L4-WB-Ocean-Map-150m-P13Y-2000-v4.0.png";
    FILE *fp = fopen(filename.c_str(), "rb");
    if (fp == nullptr) {
        log_msg("Can't open file '%s'", filename.c_str());
        return false;
    }

    png_structp png = png_create_read_struct(PNG_LIBPNG_VER_STRING, NULL, NULL, NULL);
    if(png == nullptr)
        return false;

    png_infop info = png_create_info_struct(png);
    if(info == nullptr) {
        png_destroy_read_struct(&png, nullptr, nullptr);
        return false;
    }

    //if(setjmp(png_jmpbuf(png))) abort();

    png_init_io(png, fp);

    png_read_info(png, info);

    int width = png_get_image_width(png, info);
    int height = png_get_image_height(png, info);
    png_byte color_type = png_get_color_type(png, info);
    png_byte bit_depth = png_get_bit_depth(png, info);

    log_msg("w: %d, h: %d, color_type: %d, bit_depth: %d", width, height, color_type, bit_depth);

    if (width != n_wm || height != m_wm || bit_depth != 8) {
        log_msg("Invalid map");
        return false;
    }

    log_msg("Decoded: '%s', %s", filename.c_str(), "PNG");

    // note that an automatic variable for img is optimized away
    // like png_byte img[m_wm * n_wm];
    auto img = std::make_unique<png_byte[]>(m_wm * n_wm);

    png_bytep row_pointers[m_wm];
    for(int y = 0; y < height; y++) {
        row_pointers[y] = &img[n_wm * y]; //(png_byte*)malloc(png_get_rowbytes(png,info));
    }

    png_read_image(png, row_pointers);
    fclose(fp);
    png_destroy_read_struct(&png, &info, nullptr);

    for (int i = 0; i < n_wm; i++) {
        for (int j = 10; j < m_wm - 10; j++) { // stay away from the poles
			// determined by visual adjustment
			// could be one system is at point, the other at center of grid
            int i_cs = i - 3;
            int j_cs = j - 3;

            i_cs -= n_wm / 2;
            if (i_cs < 0) {
                i_cs += n_wm;
            }

            auto is_water = [&](int i, int j) {
                j = m_wm - j; // for the image (0,0) is top left to flip y values

                int wrapped_i, wrapped_j;
                wrap_ij(i, j, wrapped_i, wrapped_j);

                png_byte pixel = (row_pointers[wrapped_j])[wrapped_i];
                return pixel == 0;
            };

            auto is_land = [&](int i, int j) {
                return !is_water(i, j);
            };

            if (is_water(i, j)) {
                wmap[i_cs][j_cs] = sWater;
				// we check whether to the opposite side is only water and in direction 'dir' is land
				// if yes we sum up all unity vectors in dir to get the 'average' direction
                float sum_x = 0.0f;
                float sum_y = 0.0f;
                bool is_coast = false;

                for (int dir = 0; dir < 8; dir++) {
                    int di = dir_x[dir];
                    int dj = dir_y[dir];
                    if (is_water(i - 2 * di, j - 2 * dj) && is_water(i - di, j - dj) && is_land(i + di, j + dj)) {
                        float f = 1.0f;
                        if (dir & 1) {
                            f = 0.7071f; // diagonal = 1/sqrt(2)
                        }
                        sum_x += f * di;
                        sum_y += f * dj;
                        is_coast = true;
                    }
                }

                if (is_coast) {
					// get angle of the average direction. We consider this as normal
					// of the coast line
                    float ang = atan2(sum_y, sum_x) / kD2R;
                    if (ang < 0) {
                        ang += 360.0f;
                    }

                    int dir_land = (int)(round(ang / 45));
                    if (dir_land == 8) {
                        dir_land = 0;
                    }

                    wmap[i_cs][j_cs] = (uint8_t)((dir_land << 2) | sCoast);
                }
            } else {
                wmap[i_cs][j_cs] = sLand;
            }
        }
    }

    int n_coast{0}, n_water{0}, n_land{0};

    for (int i = 0; i < n_wm; i++)
        for (int j = 0; j < m_wm; j++)
            if (CMIsCoast(i, j).yes_no)
                n_coast++;
            else if (coast_map.is_water(i, j))
                n_water++;
            else if (coast_map.is_land(i, j))
                n_land++;

    log_msg("coast: %d, water: %d, land: %d, probe: %d", n_coast, n_water, n_land,
            n_wm * m_wm - n_coast - n_water - n_land);

    return true;
}

// some glue for golang
extern "C"
bool CoastMapInit(const char *dir)
{
    log_msg("CoastMapInit '%s'", dir);
    return coast_map.load(dir);
}

extern "C"
bool CMIsWater(int i, int j)
{
    return coast_map.is_water(i, j);
}

extern "C"
bool CMIsLand(int i, int j)
{
    return coast_map.is_land(i, j);
}

extern "C"
R_IsCoast CMIsCoast(int i, int j)
{
    return coast_map.is_coast(i, j);
}

#ifdef TEST_COAST
// g++ -DTEST_COAST -std=c++20 -DLOCAL_DEBUGSTRING -DIBM -Wall -Werror -ISDK/CHeaders/XPLM -Iservices/ services/coast.cpp services/log_msg.cpp  -l:libpng.a -l:libz.a
int main()
{
	bool res = CoastMapInit(".");
	log_msg("res: %d", res);

    int n_coast{0}, n_water{0}, n_land{0};

    for (int i = 0; i < n_wm; i++)
        for (int j = 0; j < m_wm; j++)
            if (CMIsCoast(i, j).yes_no)
                n_coast++;
            else if (coast_map.is_water(i, j))
                n_water++;
            else if (coast_map.is_land(i, j))
                n_land++;

    log_msg("coast: %d, water: %d, land: %d, probe: %d", n_coast, n_water, n_land,
            n_wm * m_wm - n_coast - n_water - n_land);
	return 0;
}
#endif

