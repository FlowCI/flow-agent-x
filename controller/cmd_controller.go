package controller

import (
	"net/http"

	"github.com/flowci/flow-agent-x/service"
	"github.com/gin-gonic/gin"
)

type CmdController struct {
	RootController `path:"/cmds"`

	GetCmdByID gin.HandlerFunc `path:"/:id"`

	PostExecuteCmd gin.HandlerFunc `path:"/"`

	cmdService *service.CmdService
}

// NewCmdController create new instance of CmdController
func NewCmdController(router *gin.Engine) *CmdController {
	c := new(CmdController)
	c.cmdService = service.GetCmdService()

	autoWireController(c, router)
	return c
}

// GetCmdByIDImpl http get to get detail of cmd by id
func (c *CmdController) GetCmdByIDImpl(context *gin.Context) {
	id := context.Param("id")
	context.String(http.StatusOK, "id : "+id)
}

// PostExecuteCmdImpl http post request to execute cmd from request body
func (c *CmdController) PostExecuteCmdImpl(context *gin.Context) {
	bytes, err := context.GetRawData()
	if c.responseIfError(context, err) {
		return
	}

	err = c.cmdService.Execute(bytes)
	if c.responseIfError(context, err) {
		return
	}

	c.responseOk(context, nil)
}
