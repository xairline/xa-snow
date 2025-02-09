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

#ifndef _XA_SNOW_CGO_H_
#define _XA_SNOW_CGO_H_

// contains the C functions that are presented to go
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

void InitXaSnowC();
float LegacyAirportSnowDepth(float snow_depth);		// -> adjusted snow depth

// some glue for golang
bool CoastMapInit(const char *dir);
bool CMIsWater(int i, int j);
bool CMIsLand(int i, int j);

typedef
struct R_IsCoast_ {
    bool yes_no;
    int dir_x, dir_y, grid_angle;
} R_IsCoast;

R_IsCoast CMIsCoast(int i, int j);

#ifdef __cplusplus
}
#endif

#endif
