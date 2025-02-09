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

// C to golang translations that will eventually go away

// The 'old' coast.go interface

package services

import (
	"github.com/xairline/xa-snow/utils/logger"
    "unsafe"
)

// #include "xa-snow-cgo.h"
// #include <stdlib.h>
import "C"

type CoastService interface {
	IsWater(i, j int) bool
	IsLand(i, j int) bool
	IsCoast(i, j int) (bool, int, int, int)	// -> yes_no, dir_x, dir_y, grid angle
}

type coastService struct {
	logger	logger.Logger
}

func (cs *coastService)IsWater(i, j int) bool {
    return bool(C.CMIsWater(C.int(i), C.int(j)))
}

func (cs *coastService)IsLand(i, j int) bool {
    return bool(C.CMIsLand(C.int(i), C.int(j)))
}

func (cs *coastService)IsCoast(i, j int) (bool, int, int, int) {
    res := C.CMIsCoast(C.int(i), C.int(j))
    return bool(res.yes_no), int(res.dir_x), int(res.dir_y), int(res.grid_angle)
}

func NewCoastService(logger logger.Logger, dir string) CoastService {
	cs := &coastService{logger:logger}

    var cdir *C.char = C.CString(dir)
    defer C.free(unsafe.Pointer(cdir))

    if bool(C.CoastMapInit(cdir)) {
        return cs
    }

    return nil;
}

func SnowDepthToXplaneSnowNow(depth float32) (float32, float32, float32) {
    res := C.CSnowDepthToXplaneSnowNow(C.float(depth))
    return float32(res.snowNow), float32(res.snowAreaWidth), float32(res.iceNow)
}
