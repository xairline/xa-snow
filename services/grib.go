package services

import (
	"encoding/csv"
	"fmt"
	"github.com/xairline/xa-snow/utils/logger"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var gribSvcLock = &sync.Mutex{}
var gribSvc GribService

type GribService interface {
	downloadGribFile() error
	GetXplaneSnowDepth(lat, lon float32) error
	GetCalculatedSnowDepth() float32
}

type gribService struct {
	Logger              logger.Logger
	gribFilePath        string
	gribFileFolder      string
	binPath             string
	SnowDepth           float32
	SnowDepthMap        [][]float32
	SnowDepthMapCreated bool
}

func (g *gribService) GetCalculatedSnowDepth() float32 {
	if g.SnowDepth > -1 {
		return g.SnowDepth
	} else {
		return 2.0
	}
}

func (g *gribService) GetXplaneSnowDepth(lat, lon float32) error {
	if g.SnowDepthMapCreated == false {
		g.Logger.Info("Snow depth map not created yet")
		return nil
	}
	iLat := int(math.Round(float64(lat+90) * 10))
	if lon < 0 {
		lon = lon + 360
	}
	iLon := int(math.Round(float64(lon) * 10))

	g.SnowDepth = g.gribSnodToXplaneSnod(float32(math.Abs(float64(g.SnowDepthMap[iLon][iLat]))))
	if g.SnowDepth > 1.19 {
		if iLon-1 >= 0 && iLon+1 < 360 {
			g.SnowDepth = g.gribSnodToXplaneSnod(float32(math.Max(math.Abs(float64(g.SnowDepthMap[iLon-1][iLat])), math.Abs(float64(g.SnowDepthMap[iLon+1][iLat])))))
		} else if iLon-1 < 0 {
			g.SnowDepth = g.gribSnodToXplaneSnod(float32(math.Max(math.Abs(float64(g.SnowDepthMap[iLon+3599][iLat])), math.Abs(float64(g.SnowDepthMap[iLon+1][iLat])))))
		} else if iLon+1 > 3599 {
			g.SnowDepth = g.gribSnodToXplaneSnod(float32(math.Max(math.Abs(float64(g.SnowDepthMap[iLon-1][iLat])), math.Abs(float64(g.SnowDepthMap[iLon-3599][iLat])))))
		}
	}
	//g.Logger.Infof("Snow depth: %f,lon:%d,lat:%d", g.SnowDepthMap[iLon][iLat], iLon, iLat)
	return nil
}

func (g *gribService) downloadGribFile() error {
	url := getDownloadUrl()
	g.Logger.Infof("Downloading GRIB file from %s", url)
	// Get today's date in yyyy-mm-dd format
	today := time.Now().Format("2006-01-02")
	_, cycle, _ := getCycleDate()
	// Create the filename with today's date
	filename := today + "_" + fmt.Sprintf("%d", cycle) + "_noaa.grib2"
	g.gribFilePath = filepath.Join(g.gribFileFolder, filename)
	g.Logger.Infof("GRIB file path: %s", g.gribFilePath)
	// if file does not exist, download
	if _, err := os.Stat(g.gribFilePath); err != nil {
		// Get the data
		resp, err := http.Get(url)
		if err != nil {
			g.Logger.Errorf("%v", err)
			return err
		}
		defer resp.Body.Close()

		// Create the file with the date in its name
		out, err := os.Create(filename)
		if err != nil {
			g.Logger.Errorf("%v", err)
			return err
		}
		defer out.Close()

		// Write the body to file
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			g.Logger.Errorf("%v", err)
			return err
		}

		g.Logger.Info("GRIB File downloaded successfully")
	}

	g.Logger.Info("Pre-processing GRIB file")
	g.SnowDepthMapCreated = false
	//get current OS
	myOs := runtime.GOOS
	var executablePath string
	if myOs == "windows" {
		executablePath = filepath.Join(g.binPath, "WIN32wgrib2.exe")
	}
	if myOs == "linux" {
		executablePath = filepath.Join(g.binPath, "linux-wgrib2")
	}
	if myOs == "darwin" {
		executablePath = filepath.Join(g.binPath, "OSX11wgrib2")
	}
	cmd := exec.Command(executablePath, "-s", "-lola", "0:3600:0.1", "-90:1800:0.1", "snod.csv", "spread", g.gribFilePath)
	err := g.exec(cmd)
	if err != nil {
		return err
	}

	file, err := os.Open("snod.csv")
	if err != nil {
		g.Logger.Errorf("Error opening file: %v", err)
	}
	defer file.Close()

	// Create a new CSV reader
	reader := csv.NewReader(file)

	for i := 0; i < 3600; i++ {
		g.SnowDepthMap[i] = make([]float32, 1801)
	}

	counter := 0
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}
		if counter == 0 {
			counter++
			continue
		}

		// Parse longitude, latitude, and value
		lon, _ := strconv.ParseFloat(strings.TrimSpace(record[0]), 64)
		lat, _ := strconv.ParseFloat(strings.TrimSpace(record[1]), 64)
		var value float32
		if strings.Contains(record[2], "e") {
			value = 0
		}
		tmpVal, _ := strconv.ParseFloat(strings.TrimSpace(record[2]), 64)
		value = float32(tmpVal)

		// Convert longitude and latitude to array indices
		// This example assumes the CSV contains all longitudes and latitudes
		x := int(lon * 10)        // Adjust these calculations based on your data's range and resolution
		y := int((lat + 90) * 10) // Adjust for negative latitudes

		// Store the value
		g.SnowDepthMap[x][y] = value
		counter++
	}
	g.SnowDepthMapCreated = true
	g.Logger.Infof("Snow depth map size: %d", counter)
	g.Logger.Info("Pre-processing GRIB file: Done")

	g.smoothSnowDepthMap()

	//Remove old .grib files
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check for files with .grib extension
		if strings.Contains(path, "_noaa.grib2") && !strings.Contains(path, filename) {
			err := os.Remove(path)
			if err != nil {
				g.Logger.Errorf("Error removing file:", path, err)
			} else {
				g.Logger.Infof("Removed: %s", path)
			}
		}

		return nil
	})

	return nil
}

func (g *gribService) gribSnodToXplaneSnod(depth float32) float32 {
	ret := 1.2
	if depth > 0.001 {
		ret = math.Max(1.05-(1.127*math.Pow(float64(depth), 0.142)), 0.01)
	}
	return float32(ret)
}

func (g *gribService) smoothSnowDepthMap() {
	smoothedCounter := 0
	// number of boxes to smooth east or west
	smoothFactor := 20
	smoothGapMin := 3
	for i := 0; i < 1801; i++ {
		waterCounter := 0
		for j := 0; j < 3600; j++ {
			// count how many no-snow boxes we have so far
			if g.SnowDepthMap[j][i] <= 0.001 && g.SnowDepthMap[j][i] >= -0.001 {
				// if a box is missing but it's east and wst boxes are not, we fill it with the average of the two
				if j-1 >= 0 && j+1 < 3600 {
					if g.SnowDepthMap[j-1][i] > 0.001 && g.SnowDepthMap[j+1][i] > 0.001 {
						g.SnowDepthMap[j][i] = -1 * (g.SnowDepthMap[j-1][i] + g.SnowDepthMap[j+1][i]) / 2.0
					} else {
						waterCounter++
						continue
					}
				} else {
					waterCounter++
					continue
				}
			}
			if g.SnowDepthMap[j][i] > 0.001 {

				// we now find snow, if we have more than 10 no-snow boxes, we smooth the snow
				if waterCounter >= smoothGapMin {
					// smooth the snow
					for k := 1; k <= smoothFactor; k++ {
						x := j - k
						if x < 0 {
							x = 3599 + x
						}
						// only smooth it when it's not already smoothed or has snow
						if g.SnowDepthMap[x][i] < 0.001 && g.SnowDepthMap[x][i] > -0.001 {
							g.SnowDepthMap[x][i] = -1.0 * float32(math.Max(float64(float32(smoothFactor-k)*g.SnowDepthMap[j][i]/float32(smoothFactor)), 0.0))
						} else {
							break
						}
					}
					smoothedCounter += 1
					j += smoothFactor
				}
				waterCounter = 0
			}
		}
		waterCounter = 0
		for j := 3599; j >= 0; j-- {
			// count how many no-snow boxes we have so far
			if g.SnowDepthMap[j][i] <= 0.001 && g.SnowDepthMap[j][i] >= -0.001 {
				waterCounter++
				continue
			}
			if g.SnowDepthMap[j][i] > 0.001 {
				// we now find snow, if we have more than 10 no-snow boxes, we smooth the snow
				if waterCounter >= smoothGapMin {
					// smooth the snow
					for k := 1; k <= smoothFactor; k++ {
						x := j + k
						if x > 3599 {
							x = x - 3599
						}
						// only smooth it when it's not already smoothed or has snow
						if g.SnowDepthMap[x][i] < 0.001 && g.SnowDepthMap[x][i] > -0.001 {
							g.SnowDepthMap[x][i] = -1.0 * float32(math.Max(float64(float32(smoothFactor-k)*g.SnowDepthMap[j][i]/float32(smoothFactor)), 0.0))
						} else {
							break
						}
					}
					smoothedCounter += 1
					waterCounter = 0
					j -= smoothFactor
				}
			}
		}
	}
	g.Logger.Infof("Smoothed %d boxes of %f east/west", smoothedCounter, float32(smoothFactor)*0.1)
}

func NewGribService(logger logger.Logger, dir string, binPath string) GribService {
	if gribSvc != nil {
		logger.Info("Grib SVC has been initialized already")
		return gribSvc
	} else {
		logger.Info("Grib SVC: initializing")
		gribSvcLock.Lock()
		defer gribSvcLock.Unlock()

		logger.Infof("Grib SVC: initializing with folder %s", dir)

		gribSvc = &gribService{
			Logger:              logger,
			gribFileFolder:      dir,
			gribFilePath:        "",
			binPath:             binPath,
			SnowDepth:           -1.0,
			SnowDepthMap:        make([][]float32, 3600),
			SnowDepthMapCreated: false,
		}

		go func() {
			for {
				err := gribSvc.downloadGribFile()
				if err != nil {
					logger.Errorf("Download grib file failed: %v", err)
				}
				return
			}
		}()

		return gribSvc
	}
}

func getCycleDate() (string, int, int) {
	now := time.Now().UTC()
	cnow := now.Add(-4*time.Hour - 25*time.Minute) // Adjusted time considering publish delay
	cycles := []int{0, 6, 12, 18}
	var lcycle int
	for _, cycle := range cycles {
		if cnow.Hour() >= cycle {
			lcycle = cycle
		}
	}

	adjs := 0
	if cnow.Day() != now.Day() {
		adjs = 24
	}
	forecast := (adjs + now.Hour() - lcycle) / 3 * 3

	return fmt.Sprintf("%d%02d%02d", cnow.Year(), cnow.Month(), cnow.Day()), lcycle, forecast
}

func getDownloadUrl() string {
	cycleDate, cycle, forecast := getCycleDate()
	filename := fmt.Sprintf("gfs.t%02dz.pgrb2.0p25.f0%02d", cycle, forecast)
	return fmt.Sprintf("https://nomads.ncep.noaa.gov/cgi-bin/filter_gfs_0p25.pl?dir=%%2Fgfs.%s%%2F%02d%%2Fatmos&file=%s&var_SNOD=on&all_lev=on", cycleDate, cycle, filename)
	//return fmt.Sprintf("https://nomads.ncep.noaa.gov/pub/data/nccf/com/gfs/prod/gfs.%s/%02d/atmos/%s", cycleDate, cycle, filename)
}
