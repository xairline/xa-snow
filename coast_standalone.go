//go:build !test

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

func main() {
	logger := new(MyLogger)
	logger.Info("startup")
	cs := services.NewCoastService(logger, ".")
	cs.IsCoast(0, 0)

	cLand := color.NRGBA{128, 128, 128, 255}
	//cCoast := color.NRGBA{255, 0, 0, 255}
	img := image.NewNRGBA(image.Rect(0,0,3600, 1800))
	for i := 1; i < 3600; i++ {
		for j:= 0; j < 1800; j++ {
			if cs.IsLand(i, j) {
				img.SetNRGBA(i, 1800 - j, cLand)
			} else {
				yes, _, _, dir := cs.IsCoast(i, j)
				if yes {
					ang := float32(dir) * 45
					ang = 90 - ang		// for visualization use true hdg
					r, g, b := hsl.HSLtoRGBf32(ang, 1.0, 0.5)
					cCoast := color.NRGBA{uint8(r * 255), uint8(g * 255), uint8(b * 255), 255}
					img.SetNRGBA(i, 1800 - j, cCoast)
				}
			}
		}
	}

	file := "visualization.png"
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
