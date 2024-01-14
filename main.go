package main

import (
	"github.com/gin-gonic/gin"
	"github.com/xairline/goplane/extra/logging"
	"github.com/xairline/goplane/xplm/plugins"
	"github.com/xairline/goplane/xplm/utilities"
	"github.com/xairline/xa-snow/services"
	"github.com/xairline/xa-snow/utils/logger"
	"path/filepath"
)

// @BasePath  /apis

func main() {
}

func init() {
	gin.SetMode(gin.ReleaseMode)
	logger := logger.NewXplaneLogger()
	plugins.EnableFeature("XPLM_USE_NATIVE_PATHS", true)
	logging.MinLevel = logging.Info_Level
	logging.PluginName = "X Airline Snow"
	// get plugin path
	systemPath := utilities.GetSystemPath()
	pluginPath := filepath.Join(systemPath, "Resources", "plugins", "XA-snow")
	logger.Infof("Plugin path: %s", pluginPath)

	datarefSvc := services.NewDatarefService(logger)
	// entrypoint
	services.NewXplaneService(
		datarefSvc,
		logger,
	)
}
