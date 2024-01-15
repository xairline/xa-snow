package services

import (
	"fmt"
	"github.com/xairline/xa-snow/utils/logger"
	"io"
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
	Logger         logger.Logger
	gribFilePath   string
	gribFileFolder string
	binPath        string
	SnowDepth      float32
}

func (g *gribService) GetCalculatedSnowDepth() float32 {
	if g.SnowDepth > -1 {
		return g.SnowDepth
	} else {
		return 2.0
	}
}

func (g *gribService) GetXplaneSnowDepth(lat, lon float32) error {
	g.Logger.Infof("Getting snow depth from grib file: %v", g.gribFilePath)
	// get current OS
	os := runtime.GOOS
	var executablePath string
	if os == "windows" {
		executablePath = filepath.Join(g.binPath, "WIN32wgrib2.exe")
	}
	if os == "linux" {
		executablePath = filepath.Join(g.binPath, "linux-wgrib2")
	}
	if os == "darwin" {
		executablePath = filepath.Join(g.binPath, "OSX11wgrib2")
	}
	cmd := exec.Command(executablePath, "-s", "-lon", fmt.Sprintf("%f", lon), fmt.Sprintf("%f", lat), g.gribFilePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		g.Logger.Errorf("Error getting snow depth: %v,%s", err, string(output))
		return err
	}
	g.Logger.Infof("wgrib2 output: %s", output)

	strValue := strings.Split(string(output), "val=")[1]
	if strings.Contains(strValue, "e") {
		strValue = "0"
	}

	gribSnowDepth, err := strconv.ParseFloat(strings.ReplaceAll(strValue, "\n", ""), 32)
	if err != nil {
		g.Logger.Errorf("Error getting snow depth: %v", err)
		return err
	}
	g.Logger.Infof("grid snow depth(m): %f", gribSnowDepth)

	xplaneSnod := g.gribSnodToXplaneSnod(gribSnowDepth)
	g.Logger.Infof("XPlane snow depth: %f", xplaneSnod)

	g.SnowDepth = xplaneSnod
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
	// if file exists, skip download
	if _, err := os.Stat(g.gribFilePath); err == nil {
		g.Logger.Infof("File %s exists, skip download", filename)
		return nil
	}
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

func (g *gribService) gribSnodToXplaneSnod(depth float64) float32 {
	if depth > 0.3 {
		return 0.3
	}
	if depth > 0.1 {
		return 0.5
	}
	if depth > 0.05 {
		return 0.6
	}
	if depth > 0.03 {
		return 0.7
	}
	if depth > 0.01 {
		return 0.8
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
			Logger:         logger,
			gribFileFolder: dir,
			gribFilePath:   "",
			binPath:        binPath,
			SnowDepth:      -1.0,
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
	return "https://nomads.ncep.noaa.gov/cgi-bin/filter_gfs_0p50.pl?dir=%2Fgfs." + cycleDate + "%2F" + strconv.Itoa(cycle) + "%2Fatmos&file=" + filename + "&var_SNOD=on&all_lev=on"
	//return fmt.Sprintf("https://nomads.ncep.noaa.gov/pub/data/nccf/com/gfs/prod/gfs.%s/%02d/atmos/%s", cycleDate, cycle, filename)
}
