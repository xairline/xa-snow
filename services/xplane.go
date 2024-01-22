//go:build !test

package services

//go:generate mockgen -destination=./__mocks__/xplane.go -package=mocks -source=xplane.go

import (
	"github.com/xairline/goplane/extra"
	"github.com/xairline/goplane/xplm/dataAccess"
	"github.com/xairline/goplane/xplm/processing"
	"github.com/xairline/goplane/xplm/utilities"
	"github.com/xairline/xa-snow/utils/logger"
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
	Plugin          *extra.XPlanePlugin
	gribService     GribService
	datarefPointers map[string]dataAccess.DataRef
	Logger          logger.Logger
	disabled        bool
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
			Plugin: extra.NewPlugin("X Airline Snow", "com.github.xairline.xa-snow", "A plugin enables Frontend developer to contribute to xplane"),
			gribService: NewGribService(logger,
				utilities.GetSystemPath(),
				filepath.Join(utilities.GetSystemPath(), "Resources", "plugins", "XA-snow", "bin")),
			Logger:   logger,
			disabled: false,
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
		s.Logger.Infof("Plugin: %s enabled", plugin.GetName())
	case extra.PluginDisable:
		s.disabled = true
		s.Logger.Infof("Plugin: %s disabled", plugin.GetName())
	}
}

func (s *xplaneService) onPluginStart() {
	s.Logger.Info("Plugin started")
	s.datarefPointers = make(map[string]dataAccess.DataRef)

	runtime.GOMAXPROCS(runtime.NumCPU())

	lat, success := dataAccess.FindDataRef("sim/flightmodel/position/latitude")
	if !success {
		s.Logger.Error("Dataref not found")
	}
	s.datarefPointers["lat"] = lat

	lon, success := dataAccess.FindDataRef("sim/flightmodel/position/longitude")
	if !success {
		s.Logger.Error("Dataref not found")
	}
	s.datarefPointers["lon"] = lon

	processing.RegisterFlightLoopCallback(s.flightLoop, -1, nil)
}

func (s *xplaneService) onPluginStop() {
	s.Logger.Info("Plugin stopped")
}

func (s *xplaneService) flightLoop(
	elapsedSinceLastCall,
	elapsedTimeSinceLastFlightLoop float32,
	counter int,
	ref interface{},
) float32 {
	if s.datarefPointers["snow"] == nil {
		snow, success := dataAccess.FindDataRef("sim/private/controls/wxr/snow_now")
		if !success {
			s.Logger.Error("Dataref not found")
		}
		s.datarefPointers["snow"] = snow

		weatherMode, success := dataAccess.FindDataRef("sim/weather/region/weather_source")
		if !success {
			s.Logger.Error("Dataref not found")
		}
		s.datarefPointers["weatherMode"] = weatherMode
	}

	weatherMode := dataAccess.GetIntData(s.datarefPointers["weatherMode"])
	if weatherMode != 1 {
		// weather mode is not RW, we don't do anything to avoid snow on people's summer view
		return -1
	}
	if s.disabled {
		// TODO: cleanup go routines
		return 0
	}

	lat := dataAccess.GetFloatData(s.datarefPointers["lat"])
	lon := dataAccess.GetFloatData(s.datarefPointers["lon"])

	err := s.gribService.GetXplaneSnowDepth(lat, lon)
	if err != nil {
		s.Logger.Errorf("Error getting snow depth: %v", err)
	}
	snowDepth := s.gribService.GetCalculatedSnowDepth()

	dataAccess.SetFloatData(s.datarefPointers["snow"], snowDepth)

	return -1
}
