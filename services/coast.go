package services

import (
	"github.com/xairline/xa-snow/utils/logger"
	"os"
	"math"
	"path/filepath"
	"fmt"
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
	IsCoast(i, j int) (bool, int, int)
}

type coastService struct {
	logger	logger.Logger

	coastLine [n_wm][m_wm]uint8
}

func (cs *coastService)IsWater(i, j int) bool {
	return true
}

func (cs *coastService)IsLand(i, j int) bool {
	return !cs.IsWater(i,j)
}

func (cs *coastService)IsCoast(i, j int) (bool, int, int) {
	return false, 0, 0
}

func NewCoastService(logger logger.Logger, dir string) CoastService {
	file := filepath.Join(dir, "ESACCI-LC-L4-WB-Ocean-Map-150m-P13Y-2000-v4.0.png")
	reader, err := os.Open(file)
	if err != nil {
		logger.Errorf("Can't open '%s'", file)
		return nil
	}
	defer reader.Close()

	wmap, str, err := image.Decode(reader)
	if err != nil {
		logger.Errorf("Can't decode '%s'", file)
		return nil
	}
	if wmap.Bounds() != image.Rect(0, 0, n_wm, m_wm) {
		logger.Error("Invalid map")
		return nil
	}

	logger.Infof("%s %s", str, wmap.Bounds().String())

	is_water := func (i, j int) bool {
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

		r, _, _, _ := wmap.At(i, j).RGBA()
		return r == 0
	}

	is_land := func (i, j int) bool {
		return !is_water(i,j)
	}

    water := 0
	land := 0
	for i := 0; i < n_wm; i++ {
		for j := 0;  j < m_wm; j++ {
			if is_water(i, j) {
				water++
			} else {
				land++
			}
		}
	}

	fmt.Printf("w: %d, l: %d, sum: %d\n", water, land, water + land)

	var coast_dir [8]int

	for i := 0; i < n_wm; i++ {
		for j := 0;  j < m_wm; j++ {
			if is_water(i, j) {
				cgx := float32(0)
				cgy := float32(0)
				ncg := 0
				for dir := 0; dir < 8; dir++ {
					di := dir_x[dir]
					dj := dir_y[dir]
					if is_water(i-2*di, j-2*dj) && is_water(i-di, j-dj) && is_land(i+di, j+dj) {
						f := float32(1.0)
						if dir % 2 != 0 {
							f = 0.7071 // sin(45) = cos(45)
						}
						cgx += f * float32(di)
						cgy += f * float32(dj)
						ncg++
					}
				}

				if ncg > 0 {
					//if ncg == 1 {
					//	fmt.Println(i, j, cgx, cgy)
					//}
					cgx /= float32(ncg)
					cgy /= float32(ncg)

					ang := math.Atan2(float64(cgy), float64(cgx)) * 180 / math.Pi
					if ang < 0 {
						ang += 360
					}

					dir_land := int(math.Round(ang/45))
					if dir_land == 8 {
						dir_land = 0
					}

					//logger.Infof("dir_land: %d", dir_land)
					coast_dir[dir_land]++
				}
			}
		}
	}

	for k := 0; k < 8; k++ {
		fmt.Printf("dir[%d], coast: %d\n", k, coast_dir[k])
	}

	cs := &coastService{logger:logger}
	return cs
}
