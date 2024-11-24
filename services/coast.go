package services

import (
	"github.com/xairline/xa-snow/utils/logger"
	"os"
	"path/filepath"
	"fmt"
	"image"
	_ "image/png"
)

const (
	N = 1440
	M = 720
)

type CoastService interface {
	IsWater(i, j int) bool
	IsLand(i, j int) bool
	IsCoast(i, j int) (bool, int, int)
}

type coastService struct {
	logger	logger.Logger
	coastLine [N][M]uint8
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
	if wmap.Bounds() != image.Rect(0, 0, 1440, 720) {
		logger.Error("Invalid map")
		return nil
	}

	fmt.Printf("%s %s\n", str, wmap.Bounds().String())

	is_water := func (i, j int) bool {
		if i > N {
			i -= N
		}

		if i < 0 {
			i += N
		}

		if j > M {
			j = M
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
	for i := 0; i < N; i++ {
		for j := 0;  j < M; j++ {
			if is_water(i, j) {
				water++
			} else {
				land++
			}
		}
	}

	fmt.Printf("w: %d, l: %d, sum: %d\n", water, land, water + land)

	var coast_dir [8]int

	for i := 0; i < N; i++ {
		for j := 0;  j < M; j++ {
			if is_water(i, j) {
				var dir [8]int
				if is_water(i-1, j) && is_water(i-2, j) && is_land(i+1, j) {
					dir[0] = 1
					coast_dir[0]++
				}
				if is_water(i+1, j) && is_water(i+2, j) && is_land(i-1, j) {
					dir[4] = 1
					coast_dir[4]++
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
