
// The 'old' coast.go interface implemented for cgo
package services

import (
	"github.com/xairline/xa-snow/utils/logger"
)

// #include "xa-snow-cgo.h"
// #include <stdlib.h>
import "C"

import "unsafe"

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
