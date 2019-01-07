package controller

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/flowci/flow-agent-x/util"
	"github.com/gin-gonic/gin"
)

const (
	filedName    = "ControllerRoot"
	pathTagName  = "path"
	methodSuffix = "Impl"

	httpGetPrefix    = "Get"
	httpPostPrefix   = "Post"
	httpDeletePrefix = "Delete"
)

type ControllerRoot struct {
	router *gin.Engine
}

func autoWireController(c interface{}, router *gin.Engine) {
	t := util.GetType(c)

	rootPath := t.Field(0).Tag.Get(pathTagName)

	for i := 1; i < t.NumField(); i++ {
		field := t.Field(i)
		subPath := field.Tag.Get(pathTagName)

		// get field related method according to the rule
		searchMethodName := field.Name + methodSuffix
		m := reflect.ValueOf(c).MethodByName(searchMethodName)

		fmt.Println(reflect.TypeOf(m.Interface()))

		fullPath := rootPath + subPath

		if strings.HasPrefix(searchMethodName, httpGetPrefix) {
			router.GET(fullPath, toHandlerFunc(m))
			continue
		}

		if strings.HasPrefix(searchMethodName, httpPostPrefix) {
			router.POST(fullPath, toHandlerFunc(m))
			continue
		}

		if strings.HasPrefix(searchMethodName, httpDeletePrefix) {
			router.DELETE(fullPath, toHandlerFunc(m))
			continue
		}
	}
}

func toHandlerFunc(method reflect.Value) gin.HandlerFunc {
	return (method.Interface()).(func(*gin.Context))
}
