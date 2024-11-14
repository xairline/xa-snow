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
	snowDepthTabLowerLimit float32 = 0.01
	snowDepthTabUpperLimit float32 = 0.25
	snowDepthTab                   = []float32{
		snowDepthTabLowerLimit,
		0.02,
		0.03,
		0.05,
		0.10,
		0.20,
		snowDepthTabUpperLimit,
	} // depth

	snowNowTabLowerLimit float32 = 0.05
	snowNowTabUpperLimit float32 = 0.90
	snowNowTab                   = []float32{
		snowNowTabUpperLimit,
		0.70,
		0.60,
		0.30,
		0.15,
		0.06,
		snowNowTabLowerLimit,
	} // snowNow

	snowAreaWidthTabLowerLimit float32 = 0.25
	snowAreaWidthTabUpperLimit float32 = 0.33
	snowAreaWidthTab                   = []float32{
		snowAreaWidthTabUpperLimit,
		snowAreaWidthTabLowerLimit,
		snowAreaWidthTabLowerLimit,
		snowAreaWidthTabLowerLimit,
		snowAreaWidthTabLowerLimit,
		0.29,
		snowAreaWidthTabLowerLimit,
	}
)

func (p2x *phys2XPlane) SnowDepthToXplaneSnowNow(depth float32) (float32, float32) {
	if depth >= snowDepthTabUpperLimit {
		return snowNowTabLowerLimit, snowAreaWidthTabUpperLimit //snowAreaWidthTabLowerLimit
	}

	if depth <= snowDepthTabLowerLimit {
		return 1.2, snowAreaWidthTabLowerLimit //snowAreaWidthTabUpperLimit
	}

	// piecewise linear interpolation
	snowNowValue := float32(1.2)
	snowAreaWidthValue := float32(0.25)
	for i, sd0 := range snowDepthTab {
		sd1 := snowDepthTab[i+1]
		if sd0 <= depth && depth < sd1 {
			x := (depth - sd0) / (sd1 - sd0)
			snowNowValue = snowNowTab[i] + x*(snowNowTab[i+1]-snowNowTab[i])
			// apply snow area width interpolation
			// only when we have more than 20cm of snow
			if depth >= 0.2 {
				snowAreaWidthValue = snowAreaWidthTab[i] + x*(snowAreaWidthTab[i+1]-snowAreaWidthTab[i])
			}
			break
		}
	}

	return snowNowValue, snowAreaWidthValue
}
