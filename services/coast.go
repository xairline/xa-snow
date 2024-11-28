package services

import (
	"github.com/xairline/xa-snow/utils/logger"
	"os"
	"math"
	"path/filepath"
	"image"
	_ "image/png"
)

// water map in 0.1° resolution
const n_wm = 3600
const m_wm = 1800

var (
	dir_x = [8]int{1, 1, 0, -1, -1, -1,  0,  1}
	dir_y = [8]int{0, 1, 1,  1,  0, -1, -1, -1}
)


type CoastService interface {
	IsWater(i, j int) bool
	IsLand(i, j int) bool
	IsCoast(i, j int) (bool, int, int, int)	// -> yes_no, step_x, step_y, grid angle
}

const (
	sWater = iota
	sLand
	sCoast
)

type coastService struct {
	logger	logger.Logger

	wmap [n_wm][m_wm]uint8		// encoded as (dir << 2)|sXxx
}

func (cs *coastService)IsWater(i, j int) bool {
	if i >= n_wm {
		i -= n_wm
	} else if i < 0 {
		i += n_wm
	}

	if j > m_wm {
		j = m_wm
	} else if j < 0 {
		j = 0
	}

	return (cs.wmap[i][j] & 0x3) == sWater
}

func (cs *coastService)IsLand(i, j int) bool {
	if i >= n_wm {
		i -= n_wm
	} else if i < 0 {
		i += n_wm
	}

	if j > m_wm {
		j = m_wm
	} else if j < 0 {
		j = 0
	}
	return (cs.wmap[i][j] & 0x3) == sLand
}

func (cs *coastService)IsCoast(i, j int) (bool, int, int, int) {
	if j >= m_wm {
		return false, 0, 0, 0
	}

	v := cs.wmap[i][j]
	yes_no := (v & 0x3) == sCoast
	dir := v >> 2
	return yes_no, dir_x[dir], dir_y[dir], int(dir)
}

func NewCoastService(logger logger.Logger, dir string) CoastService {
	file := filepath.Join(dir, "ESACCI-LC-L4-WB-Ocean-Map-150m-P13Y-2000-v4.0.png")
	reader, err := os.Open(file)
	if err != nil {
		logger.Errorf("Can't open '%s'", file)
		return nil
	}
	defer reader.Close()

	omap, img_type, err := image.Decode(reader)
	if err != nil {
		logger.Errorf("Can't decode '%s'", file)
		return nil
	}
	if omap.Bounds() != image.Rect(0, 0, n_wm, m_wm) {
		logger.Error("Invalid map")
		return nil
	}

	logger.Infof("Decoded: '%s', %s %s", file, img_type, omap.Bounds().String())

	is_water := func (i, j int) bool {
		j = m_wm - j	// for the image (0,0) is top left to flip y values
		if i > n_wm {
			i -= n_wm
		}

		if i < 0 {
			i += n_wm
		}

		if j > m_wm {
			j = m_wm
		}

		if j < 0 {
			j = 0
		}

		r, _, _, _ := omap.At(i, j).RGBA()
		return r == 0
	}

	is_land := func (i, j int) bool {
		return !is_water(i,j)
	}

	cs := &coastService{logger:logger}

	for i := 0; i < n_wm; i++ {
		for j := 10;  j < m_wm - 10; j++ {	// stay away from the poles

			// determined by visual adjustment
			i_cs := i - 3
			j_cs := j - 3

			// xlate to grib file index
			i_cs -= n_wm/2
			if i_cs < 0 {
				i_cs += n_wm
			}

			if is_water(i, j) {
				cs.wmap[i_cs][j_cs] = sWater
				// we check whether to the opposite side is only water and in direction 'dir' is land
				// if yes we sum up all unitity vectors in dir to get the 'average' direction
				sum_x := float32(0)
				sum_y := float32(0)
				is_coast := false
				for dir := 0; dir < 8; dir++ {
					di := dir_x[dir]
					dj := dir_y[dir]
					if is_water(i-2*di, j-2*dj) && is_water(i-di, j-dj) && is_land(i+di, j+dj) {
						f := float32(1.0)
						if dir & 1 == 1 {
							f = 0.7071 // diagonal, sin(45) = cos(45) = 0.707
						}
						sum_x += f * float32(di)
						sum_y += f * float32(dj)
						is_coast = true
					}
				}

				if is_coast {
					// get angle of the average direction. We consider this as normal
					// of the coast line
					ang := math.Atan2(float64(sum_y), float64(sum_x)) * 180 / math.Pi
					if ang < 0 {
						ang += 360
					}

					// round to grid directions
					dir_land := int(math.Round(ang/45))
					if dir_land == 8 {
						dir_land = 0
					}

					cs.wmap[i_cs][j_cs] = uint8(dir_land << 2 | sCoast)
					//logger.Infof("dir_land: %d", dir_land)
				}
			} else {
				cs.wmap[i_cs][j_cs] = sLand
			}
		}
	}

	return cs
}
