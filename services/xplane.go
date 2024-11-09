//go:build !test

package services

//go:generate mockgen -destination=./__mocks__/xplane.go -package=mocks -source=xplane.go

import (
	"github.com/joho/godotenv"
	"github.com/xairline/goplane/extra"
	"github.com/xairline/goplane/xplm/dataAccess"
	"github.com/xairline/goplane/xplm/processing"
	"github.com/xairline/goplane/xplm/utilities"
	"github.com/xairline/xa-snow/utils/logger"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type XplaneService interface {
	// init
	onPluginStateChanged(state extra.PluginState, plugin *extra.XPlanePlugin)
	onPluginStart()
	onPluginStop()
	// flight loop
	flightLoop(elapsedSinceLastCall, elapsedTimeSinceLastFlightLoop float32, counter int, ref interface{}) float32
}

type xplaneService struct {
	Plugin          	*extra.XPlanePlugin
	GribService     	GribService
	p2x					Phys2XPlane
	drefsInited			bool

	lat_dr, lon_dr,
	snow_dr,
	weatherMode_dr,
	rwySnowCover_dr 	dataAccess.DataRef

	Logger          	logger.Logger
	disabled        	bool
	override        	bool

	loopCnt				uint32
	snowDepth, snowNow	float32
}

// some drefs are private and need delayed initialization
func initDrefs(s *xplaneService) bool {
	if ! s.drefsInited {
		var res bool
		success := true
		s.lat_dr, res = dataAccess.FindDataRef("sim/flightmodel/position/latitude")
		success = success && res

		s.lon_dr, res = dataAccess.FindDataRef("sim/flightmodel/position/longitude")
		success = success && res

		s.snow_dr, res = dataAccess.FindDataRef("sim/private/controls/wxr/snow_now")
		success = success && res

		s.weatherMode_dr, res = dataAccess.FindDataRef("sim/weather/region/weather_source")
		success = success && res

		s.rwySnowCover_dr, res = dataAccess.FindDataRef("sim/private/controls/twxr/snow_area_width")
		success = success && res

		if !success {
			s.Logger.Error("Dataref(s) not found")
			return false
		}
		s.drefsInited = true
	}

	return true
}

var xplaneSvcLock = &sync.Mutex{}
var xplaneSvc XplaneService

func NewXplaneService(
	logger logger.Logger,
) XplaneService {
	if xplaneSvc != nil {
		logger.Info("Xplane SVC has been initialized already")
		return xplaneSvc
	} else {
		logger.Info("Xplane SVC: initializing")
		xplaneSvcLock.Lock()
		defer xplaneSvcLock.Unlock()
		xplaneSvc := &xplaneService{
			Plugin: extra.NewPlugin("X Airline Snow", "com.github.xairline.xa-snow", "show accumulated snow in X-Plane's world"),
			GribService: NewGribService(logger,
				utilities.GetSystemPath(),
				filepath.Join(utilities.GetSystemPath(), "Resources", "plugins", "XA-snow", "bin")),
			p2x : NewPhys2XPlane(logger),
			Logger:   logger,
			disabled: false,
			override: false,
		}
		xplaneSvc.Plugin.SetPluginStateCallback(xplaneSvc.onPluginStateChanged)
		return xplaneSvc
	}
}

func (s *xplaneService) onPluginStateChanged(state extra.PluginState, plugin *extra.XPlanePlugin) {
	switch state {
	case extra.PluginStart:
		s.onPluginStart()
	case extra.PluginStop:
		s.onPluginStop()
	case extra.PluginEnable:
		s.disabled = false
		s.Logger.Infof("Plugin: %s enabled", plugin.GetName())
	case extra.PluginDisable:
		s.disabled = true
		s.Logger.Infof("Plugin: %s disabled", plugin.GetName())
		// TODO: cleanup go routines (not used now)
	}
}

func (s *xplaneService) onPluginStart() {
	s.Logger.Info("Plugin started")
	runtime.GOMAXPROCS(runtime.NumCPU())

	systemPath := utilities.GetSystemPath()
	pluginPath := filepath.Join(systemPath, "Resources", "plugins", "XA-snow")
	err := godotenv.Load(filepath.Join(pluginPath, "config"))
	if err != nil {
		s.Logger.Errorf("Some error occured. Err: %s", err)
	}
	if os.Getenv("OVERRIDE") == "true" {
		s.override = true
	}
	go func() {
		for {
			err := gribSvc.DownloadAndProcessGribFile()
			if err != nil {
				s.Logger.Errorf("Download grib file failed: %v", err)
			}
			// TODO: disabled - auto NOAA update
			return
		}
	}()

	// start with delay to let the dust settle
	processing.RegisterFlightLoopCallback(s.flightLoop, 15.0, nil)
}

func (s *xplaneService) onPluginStop() {
	s.Logger.Info("Plugin stopped")
}

// flightloop, high freq code!
func (s *xplaneService) flightLoop(
	elapsedSinceLastCall,
	elapsedTimeSinceLastFlightLoop float32,
	counter int,
	ref interface{},
) float32 {

	// delayed init
	if ! initDrefs(s) {
		return 5.0
	}

	if !s.override {
		weatherMode := dataAccess.GetIntData(s.weatherMode_dr)
		if weatherMode != 1 {
			// weather mode is not RW, we don't do anything to avoid snow on people's summer view
			return 5.0
		}
	}

	if !s.GribService.IsReady() {
		return 2.0
	}

	// throttle update computations
	s.loopCnt++
	if s.loopCnt % 8 == 1 {
		lat := dataAccess.GetFloatData(s.lat_dr)
		lon := dataAccess.GetFloatData(s.lon_dr)
		snowDepth_n := s.GribService.GetSnowDepth(lat, lon)

		// some exponential smoothing
		const alpha = float32(0.7)
		s.snowDepth = alpha *  snowDepth_n + (1 - alpha) * s.snowDepth

		// If we have no accumulated snow leave the datarefs alone and let X-Plane do its weather effects
		if s.snowDepth < 0.001 {
			return -1
		}

		s.snowNow = s.p2x.SnowDepthToXplaneSnowNow(s.snowDepth)
	}

	dataAccess.SetFloatData(s.snow_dr, s.snowNow)
	// Where I live, 40cm of snow on the ground but tarmac is clear
	// So I just blow all the snow away from the runway for you
	// consider this as a feature and not a bug
	// TODO: make this configurable
	dataAccess.SetFloatData(s.rwySnowCover_dr, 0)

	return -1
}
