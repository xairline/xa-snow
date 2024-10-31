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

const NO_SNOW = 2.0

var gribSvcLock = &sync.Mutex{}
var gribSvc GribService

type GribService interface {
	DownloadAndProcessGribFile() error
	GetXplaneSnowDepth(lat, lon float32) float32
	convertGribToMap()
	downloadGribFile() (string, error)
	gribSnodToXplaneSnowNow(depth float32) float32
}

type gribService struct {
	Logger                logger.Logger
	gribFilePath          string
	gribFileFolder        string
	binPath               string
	SnowDepthMap          [][]float32
	SnowDepthMapCreated   bool
	IceCoverageMap        [][]float32
	IceCoverageMapCreated bool
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
			Logger:                logger,
			gribFileFolder:        dir,
			gribFilePath:          "",
			binPath:               binPath,
			SnowDepthMap:          make([][]float32, 3600),
			SnowDepthMapCreated:   false,
			IceCoverageMap:        make([][]float32, 3600),
			IceCoverageMapCreated: false,
		}
		return gribSvc
	}
}

func (g *gribService) GetXplaneSnowDepth(lat, lon float32) float32 {
	if g.SnowDepthMapCreated == false {
		g.Logger.Info("Snow depth map not created yet")
		return NO_SNOW
	}
	// our snow world map is 3600x1801 [0,359.9]x[0,180.0]
	iLat := int(math.Round(float64(lat+90) * 10))

	// longitude is -180 to 180, we need to convert it to 0 to 360
	if lon < 0 {
		lon = lon + 360
	}
	iLon := int(math.Round(float64(lon) * 10))
	// if we are at the edge of the map, we use the other side of the map
	if iLon == 3600 {
		iLon = 0
	}
	return g.SnowDepthMap[iLon][iLat]
}

func (g *gribService) DownloadAndProcessGribFile() error {
	// download grib file
	gribFilename, err := g.downloadGribFile()
	if err != nil {
		return err
	}
	// convert grib file to 2D array/map
	g.convertGribToMap()
	// smooth the snow depth map
	g.smoothSnowDepthMap()
	// remove old grib files
	err = g.removeOldGribFiles(gribFilename)
	if err != nil {
		return err
	}
	return nil
}

// convert snow depth from grib(m) to xplane snow_now
func (g *gribService) gribSnodToXplaneSnowNow(depth float32) float32 {
	ret := 1.2
	if depth > 0.001 {
		ret = math.Max(1.05-(1.127*math.Pow(float64(depth), 0.102)), 0.08)
	}
	return float32(ret)
}

// smooth the snow depth map so that we don't have sudden changes
func (g *gribService) smoothSnowDepthMap() {
	smoothedCounter := 0
	// number of boxes to smooth east or west
	smoothFactor := 20
	// number of boxes to skip before we start smoothing
	// this is to avoid smoothing the snow depth map when there is no snow
	smoothGapMin := 3
	for i := 0; i < 1801; i++ {
		// smooth from west to east
		noSnowCounter := 0
		for j := 0; j < 3600; j++ {
			// count how many no-snow boxes we have so far
			if g.SnowDepthMap[j][i] <= 0.001 && g.SnowDepthMap[j][i] >= -0.001 {
				// if a box has no snow, but it's east and wst boxes has snow, we fill it with the average of the two
				if j-1 >= 0 && j+1 < 3600 {
					if g.SnowDepthMap[j-1][i] > 0.001 && g.SnowDepthMap[j+1][i] > 0.001 {
						g.SnowDepthMap[j][i] = -1 * (g.SnowDepthMap[j-1][i] + g.SnowDepthMap[j+1][i]) / 2.0
					} else {
						noSnowCounter++
						continue
					}
				} else {
					noSnowCounter++
					continue
				}
			}
			if g.SnowDepthMap[j][i] > 0.001 {
				// we now find snow,
				// if we have seen more than 10 no-snow boxes
				// we smooth the snow
				if noSnowCounter >= smoothGapMin {
					// linear smooth based on the smoothFactor
					for k := 1; k <= smoothFactor; k++ {
						x := j - k
						if x < 0 {
							x = 3599 + x
						}
						// only smooth it when it's not already smoothed or has snow
						if g.SnowDepthMap[x][i] < 0.001 && g.SnowDepthMap[x][i] > -0.001 {
							// for debugging purpose we use negative value to indicate smoothed boxes
							g.SnowDepthMap[x][i] = -1.0 * float32(math.Max(float64(float32(smoothFactor-k)*g.SnowDepthMap[j][i]/float32(smoothFactor)), 0.0))
						} else {
							break
						}
					}
					smoothedCounter += 1
					j += smoothFactor
				}
				noSnowCounter = 0
			}
		}
		// smooth from east to west
		// almost the same as the above, except we start from the east
		noSnowCounter = 0
		for j := 3599; j >= 0; j-- {
			// count how many no-snow boxes we have so far
			if g.SnowDepthMap[j][i] <= 0.001 && g.SnowDepthMap[j][i] >= -0.001 {
				noSnowCounter++
				continue
			}
			if g.SnowDepthMap[j][i] > 0.001 {
				// we now find snow, if we have more than 10 no-snow boxes, we smooth the snow
				if noSnowCounter >= smoothGapMin {
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
					noSnowCounter = 0
					j -= smoothFactor
				}
			}
		}

		// convert from grib snow depth to xplane snow_now
		for j := 0; j < 3600; j++ {
			g.SnowDepthMap[j][i] = g.gribSnodToXplaneSnowNow(g.SnowDepthMap[j][i])
		}
	}
	g.Logger.Infof("Smoothed %d boxes of %f east/west", smoothedCounter, float32(smoothFactor)*0.1)
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
	return fmt.Sprintf("https://nomads.ncep.noaa.gov/cgi-bin/filter_gfs_0p25.pl?dir=%%2Fgfs.%s%%2F%02d%%2Fatmos&file=%s&var_ICEC=on&var_SNOD=on&all_lev=on", cycleDate, cycle, filename)
	//return fmt.Sprintf("https://nomads.ncep.noaa.gov/pub/data/nccf/com/gfs/prod/gfs.%s/%02d/atmos/%s", cycleDate, cycle, filename)
}

func (g *gribService) convertGribToMap() {
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
	// export grib file to csv
	// 0:3600:0.1 means scan longitude from 0, 3600 steps with step 0.1 degree
	// -90:1800:0.1 means scan latitude from -90, 1800 steps with step 0.1 degree
	cmd := exec.Command(executablePath, "-s", "-lola", "0:3600:0.1", "-90:1800:0.1", "snod.csv", "spread", g.gribFilePath, "-match_fs", "SNOD")
	err := g.exec(cmd)
	if err != nil {
		g.Logger.Errorf("Error converting grib file: %v", err)
	}

	cmd = exec.Command(executablePath, "-s", "-lola", "0:3600:0.1", "-90:1800:0.1", "icec.csv", "spread", g.gribFilePath, "-match_fs", "ICEC")
	err = g.exec(cmd)
	if err != nil {
		g.Logger.Errorf("Error converting grib file: %v", err)
	}
	// read csv fileSnow into 2D array
	fileSnow, err := os.Open("snod.csv")
	if err != nil {
		g.Logger.Errorf("Error opening file: %v", err)
	}
	defer fileSnow.Close()
	// read csv fileSnow into 2D array
	fileIce, err := os.Open("icec.csv")
	if err != nil {
		g.Logger.Errorf("Error opening file: %v", err)
	}
	defer fileIce.Close()

	// Create a new CSV reader
	reader := csv.NewReader(fileSnow)
	for i := 0; i < 3600; i++ {
		g.SnowDepthMap[i] = make([]float32, 1801)
	}
	snowCounter := 0
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}
		if snowCounter == 0 {
			snowCounter++
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
		snowCounter++
	}

	// Create a new CSV reader
	reader = csv.NewReader(fileIce)
	for i := 0; i < 3600; i++ {
		g.IceCoverageMap[i] = make([]float32, 1801)
	}
	iceCounter := 0
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}
		if iceCounter == 0 {
			iceCounter++
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
		g.IceCoverageMap[x][y] = value
		iceCounter++
	}
	g.SnowDepthMapCreated = true
	g.IceCoverageMapCreated = true
	g.Logger.Infof("Snow depth map size: %d", snowCounter)
	g.Logger.Infof("Ice Coverage map size: %d", iceCounter)
	g.Logger.Info("Pre-processing GRIB file: Done")
}

func (g *gribService) downloadGribFile() (string, error) {
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
			return "", err
		}
		defer resp.Body.Close()

		// Create the file with the date in its name
		out, err := os.Create(filename)
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
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
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
