package syncServer

import (
	"github.com/gin-gonic/gin"

	"metacontroller/pkg/options"

	"metacontroller/pkg/controller/composite"
	"metacontroller/pkg/controller/decorator"
	"metacontroller/pkg/logging"
	"net/http"
	"strconv"
	"time"
)

type SyncServer struct {
	compositeReconciler *composite.Metacontroller
	decoratorReconciler *decorator.Metacontroller
	configuration       *options.Configuration
	syncSignal          chan bool
	gin                 *gin.Engine
}

func New(cr *composite.Metacontroller, dr *decorator.Metacontroller, configuration *options.Configuration) *SyncServer {
	return &SyncServer{cr, dr, configuration, make(chan bool, 1), gin.Default()}
}

func (r *SyncServer) sync() {
	logging.Logger.Info("Sync start")
	for _, pController := range r.compositeReconciler.ParentControllers {
		pController.Start()
	}

	for _, dController := range r.decoratorReconciler.DecoratorControllers {
		dController.Start()
	}
	time.Sleep(1 * time.Second)
	logging.Logger.Info("Sync finish")
}

func (r *SyncServer) signalListener() {
	for range r.syncSignal {
		r.sync()
	}
}

func (r *SyncServer) triggerSync(c *gin.Context) {
	select {
	case r.syncSignal <- true:
	default:
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func (r *SyncServer) Start() {
	go func() {
		port, syncErr := strconv.Atoi(r.configuration.ApiPort)
		if syncErr != nil {
			logging.Logger.Info("failed to convert apiPort to int")
			return
		}

		logging.Logger.Info("Api server starting", "port", port)

		if r.configuration.ApiTriggerSync {
			logging.Logger.Info("Api trigger_sync enabled")
			go r.signalListener()
		}

		r.gin.GET("/trigger_sync", r.triggerSync)

		if err := r.gin.Run(":" + strconv.Itoa(port)); err != nil {
			logging.Logger.Error(err, "cannot start gin server")
		}
	}()
}
