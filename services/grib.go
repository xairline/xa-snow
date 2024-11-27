package services

import (
	"github.com/xairline/xa-snow/utils/logger"
	"encoding/csv"
	"fmt"
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

// depth map of the world in 0.1° resolution
const n_iLon = 3600
const n_iLat = 1801

type DepthMap interface {
	Get(lon, lat float32) float32
	LoadCsv(csv_name string)

	// get by index with wrap around
	GetIdx(iLon, iLat int) float32
}

type depthMap struct {
	Logger  logger.Logger
	name    string
	val     [n_iLon][n_iLat]float32
	created bool
}

// grib + map service
type GribService interface {
	IsReady() bool // ready to retrieve values
	DownloadAndProcessGribFile(sys_time bool, day, month, hour int) (error, DepthMap, DepthMap) // -> err, gribSnow, coastalSnow
	GetSnowDepth(lat, lon float32) float32
	convertGribToCsv(snow_csv_name string)
	downloadGribFile(sys_time bool, day, month, hour int) (string, error)
	getDownloadUrl(sys_time bool, timeUTC time.Time) (string, time.Time, int)
	extendCoastalSnow() DepthMap
}

type gribService struct {
	Logger         logger.Logger
	gribFilePath   string
	gribFileFolder string
	binPath        string
	cs			   CoastService
	SnowDm         *depthMap
}

// load csv file into depth map
func (m *depthMap) LoadCsv(csv_name string) {
	// read csv file into 2D array
	file, err := os.Open(csv_name)
	if err != nil {
		m.Logger.Errorf("Error opening file: %v", err)
	}
	defer file.Close()

	// Create a new CSV reader
	reader := csv.NewReader(file)
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
		m.val[x][y] = value
		counter++
	}
	m.created = true
	m.Logger.Infof("%s depth map size: %d", m.name, counter)
	m.Logger.Infof("Loading CSV file '%s': Done", csv_name)
}

func (m *depthMap) GetIdx(iLon, iLat int) float32 {
	// for lon we wrap around
	if iLon >= n_iLon {
		iLon -= n_iLon
	}
	if iLon < 0 {
		iLon += n_iLon
	}

	// for lat we just confine, doesn't make a difference anyway
	if iLat > n_iLat {
		iLat = n_iLat
	}

	if iLat < 0 {
		iLat = 0
	}

	return m.val[iLon][iLat]
}

func (m *depthMap) Get(lon, lat float32) float32 {
	if !m.created {
		m.Logger.Errorf("Get called and map %s is not ready!", m.name)
		return 0.0
	}

	// our snow world map is 3600x1801 [0,359.9]x[0,180.0]
	lat += 90.0

	// longitude is -180 to 180, we need to convert it to 0 to 360
	if lon < 0 {
		lon = lon + 360
	}

	lon *= 10
	lat *= 10

	// index of tile is lower left corner
	iLon := int(lon)
	iLat := int(lat)

	// (s, t) coordinates of (lon, lat) within tile, s,t in [0,1]
	s := lon - float32(iLon)
	t := lat - float32(iLat)

	//m.Logger.Infof("(%f, %f) -> (%d, %d) (%f, %f)", lon/10, lat/10 - 90, iLon, iLat, s, t)
	v00 := m.GetIdx(iLon, iLat)
	v10 := m.GetIdx(iLon+1, iLat)
	v01 := m.GetIdx(iLon, iLat+1)
	v11 := m.GetIdx(iLon+1, iLat+1)

	// Lagrange polynoms: pij = is 1 on corner ij and 0 elsewhere
	p00 := (1 - s) * (1 - t)
	p10 := s * (1 - t)
	p01 := (1 - s) * t
	p11 := s * t

	v := v00*p00 + v10*p10 + v01*p01 + v11*p11
	//m.Logger.Infof("vij: %f, %f, %f, %f; v: %f", v00, v10, v01, v11, v)
	return v
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
			cs:				cs,
			SnowDm:         &depthMap{name: "Snow", Logger: logger},
		}
		// make sure grib file folder exists
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			os.MkdirAll(dir, os.ModePerm)
		}
		return gribSvc
	}
}

func (g *gribService) IsReady() bool {
	return g.SnowDm.created
}

func (g *gribService) GetSnowDepth(lat, lon float32) float32 {
	return g.SnowDm.Get(lon, lat)
}

func (g *gribService) extendCoastalSnow() DepthMap {
	new_dm := &depthMap{name: "Snow + Coast", Logger: g.Logger}

	const min_sd = float32(0.02)

	for i := 0; i < n_iLon; i++ {
		for j := 0; j < n_iLat; j++ {
			sd := g.SnowDm.GetIdx(i, j)
			sdn := new_dm.val[i][j]		// may already be set inland extension
			if sd > sdn {				// always maximize
				new_dm.val[i][j] = sd
			}

			if is_coast, step_x, step_y, _ := g.cs.IsCoast(i, j); is_coast && sd <= min_sd {
				// look for inland snow
				inland_dist := 0
				inland_sd := float32(0)
				for k:= 1; k <= 3; k++ {
					tmp := g.SnowDm.GetIdx(i+k*step_x, j+k*step_y)
					if tmp > sd && tmp > min_sd {
						inland_dist = k
						inland_sd = tmp
						break
					}
				}

				if (inland_dist > 0) {
					//g.Logger.Infof("Inland snow detected for (%d, %d) at dist %d, sd: %0.3f %0.3f", i, j, inland_dist, sd, inland_sd)

					// extrapolate snow ftom inland point to coast line point
					sd_base := sd
					if sd_base < min_sd {
						sd_base = min_sd
					}
					f := (inland_sd - sd_base) / float32(inland_dist)
					for k:= 0; k < inland_dist; k++ {
						new_dm.val[i+k*step_x][j+k*step_y] = sd_base + f * float32(k)
					}
				}
			}
		}
	}

	new_dm.created = true
	g.SnowDm = new_dm
	return new_dm
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

	g.SnowDm.LoadCsv(snow_csv_file)

	// remove old grib files
	err = g.removeOldGribFiles(gribFilename)
	if err != nil {
		return err, nil, nil
	}

	gribSnow := g.SnowDm
	coastalSnow := g.extendCoastalSnow()
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
