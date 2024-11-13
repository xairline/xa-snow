// This is the translation layer of the physical world (lat, lon, speed, depth, ...)
// to X Plane's dref values.
//
// It should/could be in xplane.go but would be mocked away and could not be used in
// offline testing
package services

import (
	"github.com/xairline/xa-snow/utils/logger"
)

type Phys2XPlane interface {
	SnowDepthToXplaneSnowNow(depth float32) (float32, float32)
}

type phys2XPlane struct {
	logger logger.Logger
}

func NewPhys2XPlane(logger logger.Logger) Phys2XPlane {
	return &phys2XPlane{logger}
}

// convert snow depth from grib(m) to xplane snow_now
// interpolation table
var (
	sd_tab   = []float32{0.01, 0.02, 0.03, 0.05, 0.10, 0.20, 0.40} // depth
	sn_tab   = []float32{0.90, 0.70, 0.60, 0.30, 0.15, 0.06, 0.04} // snowNow
	snaw_tab = []float32{1.60, 1.41, 1.20, 0.52, 0.24, 0.14, 0.02}
)

func (p2x *phys2XPlane) SnowDepthToXplaneSnowNow(depth float32) (float32, float32) {
	if depth >= 0.4 {
		return 0.04, 0.11
	}

	if depth <= 0.01 {
		return 1.2, 0
	}

	// piecewise linear interpolation
	snow_now_value := float32(1.2)
	snow_area_width_value := float32(0.0)
	for i, sd0 := range sd_tab {
		sd1 := sd_tab[i+1]
		if sd0 <= depth && depth < sd1 {
			x := (depth - sd0) / (sd1 - sd0)
			snow_now_value = sn_tab[i] + x*(sn_tab[i+1]-sn_tab[i])
			snow_area_width_value = snaw_tab[i] + x*(snaw_tab[i+1]-snaw_tab[i])
			break
		}
	}

	return snow_now_value, snow_area_width_value
}
