package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type CmdController struct {
	ControllerRoot `path:"/cmds"`

	GetCmdByID gin.HandlerFunc `path:"/:id"`
}

func NewCmdController(router *gin.Engine) *CmdController {
	c := &CmdController{
		ControllerRoot: ControllerRoot{
			router: router,
		},
	}

	autoWireController(c, router)
	return c
}

func (controller *CmdController) GetCmdByIDImpl(c *gin.Context) {
	id := c.Param("id")
	c.String(http.StatusOK, "id : "+id)
}
