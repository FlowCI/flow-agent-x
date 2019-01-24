package controller

import (
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
)

type HealthController struct {
	RootController `path:"/health"`

	GetInfo gin.HandlerFunc `path:"/"`
}

type HealthInfo struct {
	CPU    int              `json:"cpu"`
	Memory runtime.MemStats `json:"memory"`
}

// NewHealthController create new instance of HealthController
func NewHealthController(router *gin.Engine) *HealthController {
	c := new(HealthController)
	autoWireController(c, router)
	return c
}

func (c *HealthController) GetInfoImpl(context *gin.Context) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	info := HealthInfo{
		CPU:    runtime.NumCPU(),
		Memory: mem,
	}

	context.JSON(http.StatusOK, info)
}
