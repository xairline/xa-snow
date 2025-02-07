//go:build !test

package services

//go:generate mockgen -destination=./__mocks__/xplane.go -package=mocks -source=xplane.go

import (
	"context"
	"github.com/joho/godotenv"
	"github.com/xairline/goplane/extra"
	"github.com/xairline/goplane/xplm/dataAccess"
	"github.com/xairline/goplane/xplm/menus"
	"github.com/xairline/goplane/xplm/plugins"
	"github.com/xairline/goplane/xplm/processing"
	"github.com/xairline/goplane/xplm/utilities"
	"github.com/xairline/xa-snow/utils/logger"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"
)

// #include "xa-snow-cgo.h"
import "C"

var VERSION = "development"

type XplaneService interface {
	// init
	onPluginStateChanged(state extra.PluginState, plugin *extra.XPlanePlugin)
	onPluginStart()
	onPluginStop()
	// flight loop
	flightLoop(elapsedSinceLastCall, elapsedTimeSinceLastFlightLoop float32, counter int, ref interface{}) float32
}

type xplaneService struct {
	Plugin      *extra.XPlanePlugin
	GribService GribService
	p2x         Phys2XPlane
	drefsInited bool

	lat_dr, lon_dr,
	weatherMode_dr,
	sysTime_dr, simCurrentDay_dr, simCurrentMonth_dr, simLocalHours_dr,
	snow_dr, ice_dr,
	rwySnowCover_dr, rwyCond_dr dataAccess.DataRef

	Logger     logger.Logger
	disabled   bool
	override   bool
	rwyIce     bool
	historical bool
	autoUpdate bool
    limitSnow  bool

	loopCnt                                  uint32
	snowDepth, snowNow, iceNow, rwySnowCover float32

	myMenuId                                                              menus.MenuID
	myMenuItemIndex, myMenuItemIndex2, myMenuItemIndex3, myMenuItemIndex4, myMenuItemIndex5 int

	configFilePath string

	cancelFun context.CancelFunc

	downloadGribLock sync.Mutex
}

// private drefs need delayed initialization
func initDrefs(s *xplaneService) bool {
	if !s.drefsInited {
		var res bool
		success := true
		s.snow_dr, res = dataAccess.FindDataRef("sim/private/controls/wxr/snow_now")
		success = success && res

		s.ice_dr, res = dataAccess.FindDataRef("sim/private/controls/wxr/ice_now")
		success = success && res

		s.rwySnowCover_dr, res = dataAccess.FindDataRef("sim/private/controls/twxr/snow_area_width")
		success = success && res

		s.rwyCond_dr, res = dataAccess.FindDataRef("sim/weather/region/runway_friction")

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

		C.InitXaSnowC()	// init the C environment

		systemPath := utilities.GetSystemPath()
		pluginPath := filepath.Join(systemPath, "Resources", "plugins", "XA-snow")
		_, cancelFunc := context.WithCancel(context.Background())
		xplaneSvc := &xplaneService{
			Plugin: extra.NewPlugin("X Airline Snow - "+VERSION, "com.github.xairline.xa-snow", "show accumulated snow in X-Plane's world"),
			GribService: NewGribService(logger,
				path.Join(systemPath, "Output", "snow"),
				filepath.Join(pluginPath, "bin"),
				NewCoastService(logger, pluginPath)),
			p2x:        NewPhys2XPlane(logger),
			Logger:     logger,
			disabled:   false,
			override:   false,
			rwyIce:     true,
			historical: false,
			autoUpdate: false,
			cancelFun:  cancelFunc,
			loopCnt:    0,
		}
		xplaneSvc.Plugin.SetPluginStateCallback(xplaneSvc.onPluginStateChanged)
		xplaneSvc.Plugin.SetMessageHandler(xplaneSvc.messageHandler)
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
	s.configFilePath = filepath.Join(systemPath, "Output", "preferences", "xa-snow.prf")
	err := godotenv.Load(s.configFilePath)
	if err != nil {
		s.Logger.Warningf("Some error occured. Err: %s", err)
		s.Logger.Warning("The prf file is not required. It is used to store the state of the plugin")
	}
	if os.Getenv("OVERRIDE") == "true" {
		s.override = true
	}
	if os.Getenv("RWY_ICE") == "true" {
		s.rwyIce = true
	} else {
		s.rwyIce = false
	}
	if os.Getenv("HISTORICAL") == "true" {
		s.historical = true
	} else {
		s.historical = false
	}
	if os.Getenv("AUTOUPDATE") == "true" {
		s.autoUpdate = true
	} else {
		s.autoUpdate = false
	}

	if os.Getenv("AUTOUPDATE") == "true" {
		s.autoUpdate = true
	} else {
		s.autoUpdate = false
	}

	s.limitSnow = (os.Getenv("LIMIT_SNOW") == "true")

	// API drefs are available at plugin start
	s.lat_dr, _ = dataAccess.FindDataRef("sim/flightmodel/position/latitude")
	s.lon_dr, _ = dataAccess.FindDataRef("sim/flightmodel/position/longitude")
	s.weatherMode_dr, _ = dataAccess.FindDataRef("sim/weather/region/weather_source")
	s.sysTime_dr, _ = dataAccess.FindDataRef("sim/time/use_system_time")
	s.simCurrentMonth_dr, _ = dataAccess.FindDataRef("sim/cockpit2/clock_timer/current_month")
	s.simCurrentDay_dr, _ = dataAccess.FindDataRef("sim/cockpit2/clock_timer/current_day")
	s.simLocalHours_dr, _ = dataAccess.FindDataRef("sim/cockpit2/clock_timer/local_time_hours")

	// start with delay to let the dust settle
	processing.RegisterFlightLoopCallback(s.flightLoop, 5.0, nil)

	// setup menu
	menuId := menus.FindPluginsMenu()
	menuContainerId := menus.AppendMenuItem(menuId, "X Airline Snow", 0, false)
	s.myMenuId = menus.CreateMenu("X Airline Snow", menuId, menuContainerId, s.menuHandler, nil)
	s.myMenuItemIndex = menus.AppendMenuItem(s.myMenuId, "Toggle Override", 0, true)
	s.myMenuItemIndex2 = menus.AppendMenuItem(s.myMenuId, "Lock Elsa up (ice)", 1, true)
	s.myMenuItemIndex3 = menus.AppendMenuItem(s.myMenuId, "Enable Historical Snow", 2, true)
	s.myMenuItemIndex4 = menus.AppendMenuItem(s.myMenuId, "Enable Snow Depth Auto Update", 3, true)
    s.myMenuItemIndex5 = menus.AppendMenuItem(s.myMenuId, "Limit snow for legacy airports", 4, true)

	if s.override {
		menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex, menus.Menu_Checked)
	} else {
		menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex, menus.Menu_Unchecked)
	}
	if !s.rwyIce {
		menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex2, menus.Menu_Checked)
	} else {
		menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex2, menus.Menu_Unchecked)
	}
	if s.historical {
		menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex3, menus.Menu_Checked)
	} else {
		menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex3, menus.Menu_Unchecked)
	}
	if s.autoUpdate {
		menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex4, menus.Menu_Checked)
	} else {
		menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex4, menus.Menu_Unchecked)
	}

    m := menus.Menu_Unchecked
	if s.limitSnow { m = menus.Menu_Checked }
    menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex5, m)

	// set internal vars to known "no snow" state
	s.snowNow, s.rwySnowCover, s.iceNow = s.p2x.SnowDepthToXplaneSnowNow(0)
}

func (s *xplaneService) onPluginStop() {
	s.Logger.Info("Plugin stopped")
	s.cancelFun()
}

// flightloop, high freq code!
func (s *xplaneService) flightLoop(
	elapsedSinceLastCall,
	elapsedTimeSinceLastFlightLoop float32,
	counter int,
	ref interface{},
) float32 {

	// flightloop start is the first point in time where the time datarefs are set correctly
	if s.loopCnt == 0 {
		s.loopCnt++
		s.GribService.SetNotReady()
		s.Logger.Info("Flightloop starting, kicking off")

		// delayed init
		if !initDrefs(s) {
			return 0 // Bye, if we don't have them by now we will never get them
		}

		sys_time := dataAccess.GetIntData(s.sysTime_dr) == 1
		day := dataAccess.GetIntData(s.simCurrentDay_dr)
		month := dataAccess.GetIntData(s.simCurrentMonth_dr)
		hour := dataAccess.GetIntData(s.simLocalHours_dr)
		if !s.historical {
			s.Logger.Infof("Historical snow is enabled: %v", s.historical)
			sys_time = true
			day = time.Now().Day()
			month = int(time.Now().Month())
			hour = time.Now().Hour()
		}

		go func() {
			// Check if the mutex is locked without blocking
			s.downloadGribLock.Lock()
			s.Logger.Infof("Download grib file: lock accuired")
			defer s.downloadGribLock.Unlock()
			for i := 0; i < 3; i++ {
				err, _, _ := gribSvc.DownloadAndProcessGribFile(sys_time, month, day, hour)
				if err != nil {
					s.Logger.Errorf("Download grib file failed: %v, retry: %v", err, i)
				} else {
					s.Logger.Info("Download and process grib file successfully")
					return
				}
			}
			// if we came here, which means 3 retry failed
			// there is a problem
			s.Logger.Errorf("grib download/process: all retry failed")
			return
		}()

		return 10.0
	}

	if !s.override {
		weatherMode := dataAccess.GetIntData(s.weatherMode_dr)
		if weatherMode != 1 {
			// weather mode is not RW, we don't do anything to avoid snow on people's summer view
			return 5.0
		}
	}

	if !s.GribService.IsReady() {
		s.Logger.Warning("Processing grib data is still in progress")
		return 2.0
	}

	// throttle update computations
	s.loopCnt++
	if s.loopCnt%8 == 0 {
		lat := dataAccess.GetFloatData(s.lat_dr)
		lon := dataAccess.GetFloatData(s.lon_dr)
		snowDepth_n := s.GribService.GetSnowDepth(lat, lon)
        if s.limitSnow {
            snowDepth_n = float32(C.LegacyAirportSnowDepth(C.float(snowDepth_n)))
        }

		// some exponential smoothing
		const alpha = float32(0.7)
		s.snowDepth = alpha*snowDepth_n + (1-alpha)*s.snowDepth

		// If we have no accumulated snow leave the datarefs alone and let X-Plane do its weather effects
		if s.snowDepth < 0.001 && !s.override {
			return -1
		}

		s.snowNow, s.rwySnowCover, s.iceNow = s.p2x.SnowDepthToXplaneSnowNow(s.snowDepth)
	}

	// If we have no accumulated snow leave the datarefs alone and let X-Plane do its weather effects
	if s.snowDepth < 0.001 && !s.override {
		return -1
	}

	if !s.rwyIce {
		s.iceNow = 2
		s.rwySnowCover = 0
		dataAccess.SetFloatData(s.rwyCond_dr, 0.0)
	}

	dataAccess.SetFloatData(s.snow_dr, s.snowNow)
	dataAccess.SetFloatData(s.rwySnowCover_dr, s.rwySnowCover)
	dataAccess.SetFloatData(s.ice_dr, s.iceNow)
	rwyCond := dataAccess.GetFloatData(s.rwyCond_dr)
	if rwyCond >= 4 {
		rwyCond = rwyCond / 3
		dataAccess.SetFloatData(s.rwyCond_dr, rwyCond)
	}

	return -1
}

func (s *xplaneService) messageHandler(message plugins.Message) {
	if (message.MessageId == plugins.MSG_PLANE_LOADED || message.MessageId == plugins.MSG_SCENERY_LOADED) && s.autoUpdate {
		s.Logger.Infof("Plane/Scenery loaded: %v", message.MessageId)
		s.loopCnt = 0 // reset loop counter so we download the new grib files
	}
}

func (s *xplaneService) writeConfig() {
	// write to config
	err := godotenv.Write(map[string]string{
		"OVERRIDE":   strconv.FormatBool(s.override),
		"RWY_ICE":    strconv.FormatBool(s.rwyIce),
		"HISTORICAL": strconv.FormatBool(s.historical),
		"AUTOUPDATE": strconv.FormatBool(s.autoUpdate),
        "LIMIT_SNOW": strconv.FormatBool(s.limitSnow),
	}, s.configFilePath)
	if err != nil {
		s.Logger.Errorf("Error writing to config: %v", err)
	}
}


func (s *xplaneService) menuHandler(menuRef interface{}, itemRef interface{}) {
	if itemRef.(int) == 0 {
		s.override = !s.override

		if s.override {
			menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex, menus.Menu_Checked)
		} else {
			menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex, menus.Menu_Unchecked)

		}
		s.Logger.Infof("Override: %v", s.override)
	}
	if itemRef.(int) == 1 {
		s.rwyIce = !s.rwyIce
		if s.rwyIce {
			menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex2, menus.Menu_Unchecked)
		} else {
			menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex2, menus.Menu_Checked)

		}
		s.Logger.Infof("Rwy Ice: %v", s.rwyIce)
	}
	if itemRef.(int) == 2 {
		s.historical = !s.historical
		if s.historical {
			menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex3, menus.Menu_Checked)
		} else {
			menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex3, menus.Menu_Unchecked)

		}
		s.Logger.Infof("Historical: %v", s.historical)
	}
	if itemRef.(int) == 3 {
		s.autoUpdate = !s.autoUpdate
		if s.autoUpdate {
			menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex4, menus.Menu_Checked)
		} else {
			menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex4, menus.Menu_Unchecked)

		}
		s.Logger.Infof("NOAA Auto update: %v", s.autoUpdate)
	}

	if itemRef.(int) == 4 {
		s.limitSnow = !s.limitSnow
		if s.limitSnow {
			menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex5, menus.Menu_Checked)
		} else {
			menus.CheckMenuItem(s.myMenuId, s.myMenuItemIndex5, menus.Menu_Unchecked)

		}
		s.Logger.Infof("LIMIT_SNOW: %v", s.limitSnow)
	}

    s.writeConfig()
}
