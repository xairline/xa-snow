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

// depth map of the world in 0.1° resolution
const n_iLon = 3600
const n_iLat = 1801

type DepthMap interface {
	Get(lon, lat float32) float32
	LoadCsv(csv_name string)
	Smooth()
}

type depthMap struct {
	Logger logger.Logger
	name string
	val [n_iLon][n_iLat]float32
	created bool
}

// grib + map service
type GribService interface {
    IsReady() bool                                          // ready to retrieve values
	DownloadAndProcessGribFile() error
	GetSnowDepth(lat, lon float32) float32
	convertGribToCsv(snow_csv_name, ice_csv_name string)
	downloadGribFile() (string, error)
}

type gribService struct {
	Logger                logger.Logger
	gribFilePath          string
	gribFileFolder        string
	binPath               string
	SnowDm, IceDm         *depthMap
}

// load csv file into depth map
func (m *depthMap)LoadCsv(csv_name string) {
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

func (m *depthMap)Get(lon, lat float32) float32 {
	if !m.created {
		m.Logger.Errorf("Get called and map %s is not ready!", m.name)
		return 0.0
	}
	// our snow world map is 3600x1801 [0,359.9]x[0,180.0]
	iLat := int(math.Round(float64(lat+90) * 10))

	// longitude is -180 to 180, we need to convert it to 0 to 360
	if lon < 0 {
		lon = lon + 360
	}
	iLon := int(math.Round(float64(lon) * 10))
	// if we are at the edge of the map, we use the other side of the map
	if iLon == n_iLon {
		iLon = 0
	}
	return m.val[iLon][iLat]
}

// legacy smoothin algorithm
// smooth the snow depth map so that we don't have sudden changes
func (m *depthMap)Smooth() {
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
			if m.val[j][i] <= 0.001 && m.val[j][i] >= -0.001 {
				// if a box has no snow, but it's east and wst boxes has snow, we fill it with the average of the two
				if j-1 >= 0 && j+1 < 3600 {
					if m.val[j-1][i] > 0.001 && m.val[j+1][i] > 0.001 {
						m.val[j][i] = -1 * (m.val[j-1][i] + m.val[j+1][i]) / 2.0
					} else {
						noSnowCounter++
						continue
					}
				} else {
					noSnowCounter++
					continue
				}
			}
			if m.val[j][i] > 0.001 {
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
						if m.val[x][i] < 0.001 && m.val[x][i] > -0.001 {
							// for debugging purpose we use negative value to indicate smoothed boxes
							m.val[x][i] = -1.0 * float32(math.Max(float64(float32(smoothFactor-k)*m.val[j][i]/float32(smoothFactor)), 0.0))
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
			if m.val[j][i] <= 0.001 && m.val[j][i] >= -0.001 {
				noSnowCounter++
				continue
			}
			if m.val[j][i] > 0.001 {
				// we now find snow, if we have more than 10 no-snow boxes, we smooth the snow
				if noSnowCounter >= smoothGapMin {
					// smooth the snow
					for k := 1; k <= smoothFactor; k++ {
						x := j + k
						if x > 3599 {
							x = x - 3599
						}
						// only smooth it when it's not already smoothed or has snow
						if m.val[x][i] < 0.001 && m.val[x][i] > -0.001 {
							m.val[x][i] = -1.0 * float32(math.Max(float64(float32(smoothFactor-k)*m.val[j][i]/float32(smoothFactor)), 0.0))
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
	}
	m.Logger.Infof("Smoothed %d boxes of %f east/west", smoothedCounter, float32(smoothFactor)*0.1)
}

var gribSvcLock = &sync.Mutex{}
var gribSvc GribService

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
            SnowDm:                &depthMap{ name: "Snow", Logger: logger },
            IceDm:                 &depthMap{ name: "Ice", Logger: logger },
		}
		return gribSvc
	}
}

func (g *gribService) IsReady() bool {
    return g.SnowDm.created && g.IceDm.created
}

func (g *gribService) GetSnowDepth(lat, lon float32) float32 {
    return g.SnowDm.Get(lon, lat)
}

func (g *gribService) DownloadAndProcessGribFile() error {
    file_override := 0

    snow_csv_file := "snod.csv"
    ice_csv_file := "icec.csv"

    tmp := os.Getenv("USE_SNOD_CSV")
    if tmp != "" {
        snow_csv_file = tmp
        file_override++
    }

    tmp = os.Getenv("USE_ICEC_CSV")
    if tmp != "" {
        ice_csv_file = tmp
        file_override++
    }

    var gribFilename string
    var err error

    if (file_override < 2) {
        // download grib file
        gribFilename, err = g.downloadGribFile()
        if err != nil {
            return err
        }
        // convert grib file to csv files
        g.convertGribToCsv("snod.csv", "icec.csv")
    }

    g.SnowDm.LoadCsv(snow_csv_file)
    g.IceDm.LoadCsv(ice_csv_file)

	// smooth the snow depth map
    g.SnowDm.Smooth()
	// remove old grib files
	err = g.removeOldGribFiles(gribFilename)
	if err != nil {
		return err
	}
	return nil
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

func (g *gribService) convertGribToCsv(snow_csv_name, ice_csv_name string) {
	g.Logger.Infof("Pre-processing GRIB file to CSV: '%s' and '%s'", snow_csv_name, ice_csv_name)
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

	cmd = exec.Command(executablePath, "-s", "-lola", "0:3600:0.1", "-90:1800:0.1", ice_csv_name, "spread", g.gribFilePath, "-match_fs", "ICEC")
	err = g.exec(cmd)
	if err != nil {
		g.Logger.Errorf("Error converting grib file: %v", err)
	}
	g.Logger.Info("Pre-processing GRIB file to CSV: Done")
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
