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
	iLat := int(math.Round(float64(lat + 90)))
	if lon < 0 {
		lon = lon + 360
	}
	iLon := int(math.Round(float64(lon)))

	g.SnowDepth = g.gribSnodToXplaneSnod(g.SnowDepthMap[iLon][iLat])
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
	cmd := exec.Command(executablePath, "-s", "-lola", "0:720:1", "-90:360:1", "snod.csv", "spread", g.gribFilePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		g.Logger.Errorf("Error getting snow depth: %v,%s", err, string(output))
		return err
	}

	file, err := os.Open("snod.csv")
	if err != nil {
		g.Logger.Errorf("Error opening file: %v", err)
	}
	defer file.Close()

	// Create a new CSV reader
	reader := csv.NewReader(file)

	for i := 0; i < 360; i++ {
		g.SnowDepthMap[i] = make([]float32, 181)
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
		x := int(lon)      // Adjust these calculations based on your data's range and resolution
		y := int(lat + 90) // Adjust for negative latitudes

		// Store the value
		g.SnowDepthMap[x][y] = value
		counter++
	}
	g.SnowDepthMapCreated = true
	g.Logger.Infof("Snow depth map size: %d", counter)
	g.Logger.Info("Pre-processing GRIB file: Done")

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
	if depth > 0.3 {
		return 0.1
	}
	if depth > 0.1 {
		return 0.2
	}
	if depth > 0.05 {
		return 0.3
	}
	if depth > 0.03 {
		return 0.35
	}
	if depth > 0.01 {
		return 0.4
	}
	if depth > 0.0 {
		return 0.9
	}
	return 1.2
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
			SnowDepthMap:        make([][]float32, 360),
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
	filename := fmt.Sprintf("gfs.t%02dz.pgrb2full.0p50.f0%02d", cycle, forecast)
	return fmt.Sprintf("https://nomads.ncep.noaa.gov/cgi-bin/filter_gfs_0p50.pl?dir=%%2Fgfs.%s%%2F%02d%%2Fatmos&file=%s&var_SNOD=on&all_lev=on", cycleDate, cycle, filename)
	//return fmt.Sprintf("https://nomads.ncep.noaa.gov/pub/data/nccf/com/gfs/prod/gfs.%s/%02d/atmos/%s", cycleDate, cycle, filename)
}
