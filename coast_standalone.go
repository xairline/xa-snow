//go:build ignore

package main

import (
	"fmt"
	"os"
	"github.com/xairline/xa-snow/services"
	"image"
	"image/color"
	"image/png"
	"goki.dev/cam/hsl"
)

// MyLogger is a mock type for the Logger type
type MyLogger struct {
	a int
}

func (m *MyLogger) Info(msg string) {
	fmt.Println("Info:", msg)
}

func (m *MyLogger) Debugf(format string, a ...interface{}) {
	fmt.Println("Debug:", fmt.Sprintf(format, a...))
}

func (m *MyLogger) Debug(msg string) {
	fmt.Println(msg)
}

func (m *MyLogger) Error(msg string) {
	fmt.Println(msg)
}

func (m *MyLogger) Warningf(format string, a ...interface{}) {
	fmt.Println("Warning:", fmt.Sprintf(format, a...))
}

func (m *MyLogger) Warning(msg string) {
	fmt.Println("Warning:", msg)
}

func (m *MyLogger) Infof(format string, a ...interface{}) {
	fmt.Println("Info:", fmt.Sprintf(format, a...))
}

func (m *MyLogger) Errorf(format string, a ...interface{}) {
	fmt.Println("Error:", fmt.Sprintf(format, a...))
}

var sm, sm_coast services.DepthMap

func logSnow(loc string, lat, lon float32) {
	fmt.Printf("at %s, grib sd: %0.3f, coastal sd: %0.3f\n", loc, sm.Get(lon, lat), sm_coast.Get(lon, lat))
}

func main() {
	logger := new(MyLogger)
	logger.Info("startup")
	cs := services.NewCoastService(".")

	img := image.NewNRGBA(image.Rect(0,0,3600, 1800))

	gs := services.NewGribService(logger, ".", "bin", cs)
	//_, sm, sm_coast = gs.DownloadAndProcessGribFile(false, 12, 03, 18)
	_, sm, sm_coast = gs.DownloadAndProcessGribFile(true, 12, 03, 18)

	logSnow("ESGG", 57.650, 12.268)
	logSnow("ESGG coast", 57.668, 11.934)

	logSnow("ENBR", 60.280, 5.222)
	logSnow("ENBR coast", 60.271421, 4.952735)

	// land
	if true {
		cLand := color.NRGBA{80, 80, 80, 255}
		for i := 0; i < 3600; i++ {
			for j:= 0; j < 1800; j++ {
				if cs.IsLand(i, j) {
					img.SetNRGBA(i, 1800 - j, cLand)
				}
			}
		}
	}

	// snow
	if true {
		for i := 0; i < 3600; i++ {
			for j:= 0; j < 1800; j++ {
				sd := sm.GetIdx(i, j)

				if sd > 0.01 {
					const sd_max = 0.10
					if sd > sd_max {
						sd = sd_max
					}
					sd = sd / sd_max
					const ofs = 70
					cSnow := color.NRGBA{0, ofs + uint8(sd * (255 -ofs)), ofs + uint8(sd * (255 - ofs)), 255}
					img.SetNRGBA(i, 1800 - j, cSnow)
				}
			}
		}
	}

	// coastal snow
	for i := 0; i < 3600; i++ {
		for j:= 0; j < 1800; j++ {
 			sd := sm.GetIdx(i, j)
 			sdc := sm_coast.GetIdx(i, j)
			if sd != sdc {
				//logger.Infof("%d, %d, %0.3f, %0.3f", i, j, sd, sdc)
				const ofs = 100
				cSnow := color.NRGBA{ofs + uint8(sdc * (255 -ofs)), ofs + uint8(sdc * (255 - ofs)), 0, 255}
				img.SetNRGBA(i, 1800 - j, cSnow)
			}
		}
	}

	// coast line
	if false {
		for i := 0; i < 3600; i++ {
			for j:= 0; j < 1800; j++ {
				if yes, _, _, dir := cs.IsCoast(i, j); yes {
					ang := float32(dir) * 45
					ang = 90 - ang		// for visualization use true hdg
					r, g, b := hsl.HSLtoRGBf32(ang, 1.0, 0.5)
					cCoast := color.NRGBA{uint8(r * 255), uint8(g * 255), uint8(b * 255), 255}
					img.SetNRGBA(i, 1800 - j, cCoast)
				}
			}
		}
	}

	file := "coast_visualization.png"
	f, err := os.Create(file)
	if err != nil {
		logger.Errorf("Can't open '%s'", file)
		return
	}
	defer f.Close()

	err = png.Encode(f, img)
	if err != nil {
		logger.Error("Encode failed")
	}
}
