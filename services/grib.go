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
	"strings"
	"sync"
	"time"
)

// grib service
type GribService interface {
	IsReady() bool                                                                              // ready to retrieve values
	DownloadAndProcessGribFile(sys_time bool, day, month, hour int) (error, DepthMap, DepthMap) // -> err, gribSnow, coastalSnow
	GetSnowDepth(lat, lon float32) float32
	convertGribToCsv(snow_csv_name string)
	downloadGribFile(sys_time bool, day, month, hour int) (string, error)
	getDownloadUrl(sys_time bool, timeUTC time.Time) (string, time.Time, int)
	SetNotReady()
}

type gribService struct {
	ready          bool
	Logger         logger.Logger
	gribFilePath   string
	gribFileFolder string
	binPath        string
	cs             CoastService
	SnowDm         DepthMap
}

func (g *gribService) SetNotReady() {
	g.ready = false
}

var gribSvcLock = &sync.Mutex{}
var gribSvc GribService

func NewGribService(logger logger.Logger, dir string, binPath string, cs CoastService) GribService {
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
			cs:             cs,
		}
		// make sure grib file folder exists
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			os.MkdirAll(dir, os.ModePerm)
		}
		return gribSvc
	}
}

func (g *gribService) IsReady() bool {
	return g.ready
}

func (g *gribService) GetSnowDepth(lat, lon float32) float32 {
	if !g.ready {
		g.Logger.Error("Get called and service is not ready!")
		return 0.0
	}

	return g.SnowDm.Get(lon, lat)
}

func (g *gribService) DownloadAndProcessGribFile(sys_time bool, month, day, hour int) (error, DepthMap, DepthMap) {
	file_override := 0

	snow_csv_file := "snod.csv"

	tmp := os.Getenv("USE_SNOD_CSV")
	if tmp != "" {
		snow_csv_file = tmp
		file_override++
	}

	var gribFilename string
	var err error

	if file_override < 1 {
		// download grib file
		gribFilename, err = g.downloadGribFile(sys_time, day, month, hour)
		if err != nil {
			return err, nil, nil
		}
		// convert grib file to csv files
		g.convertGribToCsv("snod.csv")
	}

	gribSnow := NewDepthMap()
	gribSnow.LoadCsv(snow_csv_file)

	// remove old grib files
	err = g.removeOldGribFiles(gribFilename)
	if err != nil {
		return err, nil, nil
	}

	coastalSnow := ElsaOnTheCoast(gribSnow)
	g.SnowDm = coastalSnow
	g.ready = true
	return nil, gribSnow, coastalSnow
}

func (g *gribService) getDownloadUrl(sys_time bool, timeUTC time.Time) (string, time.Time, int) {
	g.Logger.Infof("timeUTC:  %s", timeUTC.String())
	ctimeUTC := timeUTC.Add(-4*time.Hour - 25*time.Minute) // Adjusted time considering publish delay
	g.Logger.Infof("ctimeUTC: %s", ctimeUTC.String())
	cycles := []int{0, 6, 12, 18}
	var cycle int
	for _, cycle_ := range cycles {
		if ctimeUTC.Hour() >= cycle_ {
			cycle = cycle_
		}
	}

	adjs := 0
	if ctimeUTC.Day() != timeUTC.Day() {
		adjs = 24
	}
	forecast := (adjs + timeUTC.Hour() - cycle) / 3 * 3

	cycleDate := fmt.Sprintf("%d%02d%02d", ctimeUTC.Year(), ctimeUTC.Month(), ctimeUTC.Day())

	if sys_time {
		filename := fmt.Sprintf("gfs.t%02dz.pgrb2.0p25.f0%02d", cycle, forecast)
		g.Logger.Infof("NOAA Filename: %s, %d, %d", filename, cycle, forecast)
		url := fmt.Sprintf("https://nomads.ncep.noaa.gov/cgi-bin/filter_gfs_0p25.pl?dir=%%2Fgfs.%s%%2F%02d%%2Fatmos&file=%s&var_SNOD=on&all_lev=on", cycleDate, cycle, filename)
		return url, ctimeUTC, cycle
	} else {
		forecast = 6 // TODO: for now
		filename := fmt.Sprintf("gfs.0p25.%s%02d.f0%02d.grib2", cycleDate, cycle, forecast)
		g.Logger.Infof("GITHUB Filename: %s, %d, %d", filename, cycle, forecast)
		url := fmt.Sprintf("https://github.com/xairline/weather-data/releases/download/daily/%s", filename)
		return url, ctimeUTC, cycle
	}

	//return fmt.Sprintf("https://nomads.ncep.noaa.gov/pub/data/nccf/com/gfs/prod/gfs.%s/%02d/atmos/%s", cycleDate, cycle, filename)
}

func (g *gribService) convertGribToCsv(snow_csv_name string) {
	g.Logger.Infof("Pre-processing GRIB file to CSV: '%s'", snow_csv_name)
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
	// export grib file to csv
	// 0:3600:0.1 means scan longitude from 0, 3600 steps with step 0.1 degree
	// -90:1800:0.1 means scan latitude from -90, 1800 steps with step 0.1 degree
	cmd := exec.Command(executablePath, "-s", "-lola", "0:3600:0.1", "-90:1800:0.1", snow_csv_name, "spread", g.gribFilePath, "-match_fs", "SNOD")
	err := g.exec(cmd)
	if err != nil {
		g.Logger.Errorf("Error converting grib file: %v", err)
	}

	g.Logger.Info("Pre-processing GRIB file to CSV: Done")
}

// day, month, hour are in the local TZ
func (g *gribService) downloadGribFile(sys_time bool, day, month, hour int) (string, error) {
	g.Logger.Infof("downloadGribFile: Using system time: %t, month: %d, day: %d, hour: %d",
		sys_time, month, day, hour)

	now := time.Now()
	timeUTC := now.UTC()
	// Convert the provided day, month, and hour into a time.Time object
	providedTime := time.Date(now.Year(), time.Month(month), day, hour, 0, 0, 0, time.UTC)

	// Check if the provided time is within the last 24 hours
	if providedTime.After(now.Add(-24*time.Hour)) && providedTime.Before(now) && !sys_time {
		g.Logger.Infof("The provided time is within the last 24 hours. Using system time.")
		sys_time = true
	}

	if !sys_time {
		// historic mode

		loc := now.Location() // my TZ
		year := now.Year()

		m := int(now.Month())
		if (month > m) ||
			(month == m && day > now.Day()) ||
			(month == m && day == now.Day() && hour > now.Hour()) {
			// future month/day/hour -> use previous year
			year--
		}

		timeUTC = time.Date(year, time.Month(month), day, hour, 0, 0, 0, loc).UTC()
	}

	url, ctimeUTC, cycle := g.getDownloadUrl(sys_time, timeUTC)
	g.Logger.Infof("Downloading GRIB file from %s", url)
	// Get today's date in yyyy-mm-dd format
	today := ctimeUTC.Format("2006-01-02")
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
			return "", err
		}
		defer resp.Body.Close()

		// Create the file with the date in its name
		out, err := os.Create(g.gribFilePath)
		if err != nil {
			g.Logger.Errorf("%v", err)
			return "", err
		}
		defer out.Close()
		if resp.StatusCode != 200 {
			g.Logger.Errorf("Failed to download GRIB file: %s", resp.Status)
		}

		// Write the body to file
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			g.Logger.Errorf("%v", err)
			return "", err
		}
		g.Logger.Info("GRIB File downloaded successfully")
	}
	return filename, nil
}

func (g *gribService) removeOldGribFiles(fileToKeep string) error {
	//Remove old .grib files
	g.Logger.Info("Removing old grib files")
	g.Logger.Infof("File to keep: %s", fileToKeep)
	g.Logger.Infof("Grib file folder: %s", g.gribFileFolder)
	err := filepath.Walk(g.gribFileFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(path, "Scenery") {
			return nil
		}

		// Check for files with .grib extension
		if strings.Contains(path, "_noaa.grib2") && !strings.Contains(path, fileToKeep) {
			err := os.Remove(path)
			if err != nil {
				g.Logger.Errorf("Error removing file:", path, err)
			} else {
				g.Logger.Infof("Removed: %s", path)
			}
		}

		return nil
	})
	if err != nil {
		g.Logger.Errorf("Error removing old grib files: %v", err)
		return err
	}
	return nil
}
