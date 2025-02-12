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

package services

// #include "xa-snow-cgo.h"
// #include <stdlib.h>
import "C"

//----------------------------------------------------------------------------------
func SnowDepthToXplaneSnowNow(depth float32) (float32, float32, float32) {
    res := C.CSnowDepthToXplaneSnowNow(C.float(depth))
    return float32(res.snowNow), float32(res.snowAreaWidth), float32(res.iceNow)
}
