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
	SnowDepthToXplaneSnowNow(depth float32) (float32, float32, float32)
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
	snowDepthTab     = []float32{0.01, 0.02, 0.03, 0.05, 0.10, 0.20, 0.25}
	snowNowTab       = []float32{0.90, 0.70, 0.60, 0.30, 0.15, 0.06, 0.05}
	snowAreaWidthTab = []float32{0.25, 0.25, 0.25, 0.25, 0.25, 0.29, 0.33}
	iceNowTab        = []float32{2.00, 2.00, 2.00, 2.00, 0.80, 0.37, 0.37}
)

func (p2x *phys2XPlane) SnowDepthToXplaneSnowNow(depth float32) (float32, float32, float32) {
	if depth >= snowDepthTab[len(snowDepthTab)-1] {
		return snowNowTab[len(snowNowTab)-1], snowAreaWidthTab[len(snowAreaWidthTab)-1],
			   iceNowTab[len(iceNowTab)-1]
	}

	if depth <= snowDepthTab[0] {
		return 1.2, snowAreaWidthTab[0], iceNowTab[0]
	}

	// piecewise linear interpolation
	snowNowValue := float32(1.2)
	iceNowValue := iceNowTab[0]
	snowAreaWidthValue := snowAreaWidthTab[0]

	for i, sd0 := range snowDepthTab {
		sd1 := snowDepthTab[i+1]
		if sd0 <= depth && depth < sd1 {
			x := (depth - sd0) / (sd1 - sd0)
			snowNowValue = snowNowTab[i] + x*(snowNowTab[i+1]-snowNowTab[i])
			snowAreaWidthValue = snowAreaWidthTab[i] + x*(snowAreaWidthTab[i+1]-snowAreaWidthTab[i])
			iceNowValue = iceNowTab[i] + x*(iceNowTab[i+1]-iceNowTab[i])
			break
		}
	}

	return snowNowValue, snowAreaWidthValue, iceNowValue
}
