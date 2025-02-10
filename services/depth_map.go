package services

import (
	"encoding/csv"
	"github.com/xairline/xa-snow/utils/logger"
	"os"
	"strconv"
	"strings"
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
	Logger logger.Logger
	name   string
	val    [n_iLon][n_iLat]float32
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
	m.Logger.Infof("%s depth map size: %d", m.name, counter)
	m.Logger.Infof("Loading CSV file '%s': Done", csv_name)
}

func (m *depthMap) GetIdx(iLon, iLat int) float32 {
	// for lon we wrap around
	if iLon >= n_iLon {
		iLon -= n_iLon
	} else if iLon < 0 {
		iLon += n_iLon
	}

	// for lat we just confine, doesn't make a difference anyway
	if iLat >= n_iLat {
		iLat = n_iLat - 1
	} else if iLat < 0 {
		iLat = 0
	}

	return m.val[iLon][iLat]
}

func (m *depthMap) Get(lon, lat float32) float32 {
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

func ElsaOnTheCoast(gribSnow *depthMap, cs CoastService) DepthMap {
	new_dm := &depthMap{name: "Snow + Coast", Logger: gribSnow.Logger}

	const min_sd = float32(0.02) // only go higher than this snow depth

	n_extend := 0

	for i := 0; i < n_iLon; i++ {
		for j := 0; j < n_iLat; j++ {
			sd := gribSnow.GetIdx(i, j)
			sdn := new_dm.val[i][j] // may already be set by inland extension earlier
			if sd > sdn {           // always maximize
				new_dm.val[i][j] = sd
			}

			const max_step = 3 // to look for inland snow ~ 5 to 10 km / step
			if is_coast, dir_x, dir_y, _ := cs.IsCoast(i, j); is_coast && sd <= min_sd {
				// look for inland snow
				inland_dist := 0
				inland_sd := float32(0)
				for k := 1; k <= max_step; k++ {
					ii := i + k*dir_x
					jj := j + k*dir_y

					if k < max_step && cs.IsWater(ii, jj) { // if possible skip water
						continue
					}

					tmp := gribSnow.GetIdx(ii, jj)
					if tmp > sd && tmp > min_sd { // found snow
						inland_dist = k
						inland_sd = tmp
						break
					}
				}

				const decay = float32(0.8) // snow depth decay per step
				if inland_dist > 0 {
					//g.Logger.Infof("Inland snow detected for (%d, %d) at dist %d, sd: %0.3f %0.3f",
					//				 i, j, inland_dist, sd, inland_sd)

					// use power law from inland point to coast line point
					for k := inland_dist - 1; k >= 0; k-- {
						inland_sd *= decay
						if inland_sd < min_sd {
							inland_sd = min_sd
						}
						x := i + k*dir_x
						y := j + k*dir_y
						if x >= n_iLon {
							x -= n_iLon
						}
						if x < 0 {
							x += n_iLon
						}

                        // the poles are tricky so we just clamp
                        // anyway it does not make a difference
						if y >= n_iLat {
							y = n_iLat
						}
						if y < 0 {
							y = 0
						}
						new_dm.val[x][y] = inland_sd
						n_extend++
					}
				}
			}
		}
	}

	new_dm.Logger.Infof("Extended costal snow on %d grid points", n_extend)
	return new_dm
}

