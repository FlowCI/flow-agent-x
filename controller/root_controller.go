package controller

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/flowci/flow-agent-x/util"
	"github.com/gin-gonic/gin"
)

const (
	filedName    = "RootController"
	pathTagName  = "path"
	methodSuffix = "Impl"

	httpGetPrefix    = "Get"
	httpPostPrefix   = "Post"
	httpDeletePrefix = "Delete"
)

// ResponseMessage the response body
type ResponseMessage struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// RootController supper controller type
type RootController struct {
}

func (c *RootController) responseIfError(context *gin.Context, err error) bool {
	if err == nil {
		return false
	}

	context.Abort()

	context.JSON(http.StatusBadRequest, ResponseMessage{
		Code:    -1,
		Message: err.Error(),
	})

	return context.IsAborted()
}

func (c *RootController) responseOk(context *gin.Context, data interface{}) {
	context.JSON(http.StatusOK, ResponseMessage{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

// autoWireController it will regist request mapping automatically
//
// Example:
// 	type SubController struct {
//		RootController 		 	'path:"/roots"'
//
//		PostCreateHelloWorld 	gin.HandlerFunc 'path:"/new"'
//  }
//
// The request '/roots/new'	for http POST will be registered
//
// first field 'RootController' define the root path by tag 'path'
//
// the field 'PostCreateHelloWorld' define POST request by method prefix
// and sub request mapping by tag 'path'
//
// the method 'PostCreateHelloWorldImpl' has to created to receive the request
func autoWireController(c interface{}, router *gin.Engine) {
	t := util.GetType(c)

	rootPath := t.Field(0).Tag.Get(pathTagName)

	for i := 1; i < t.NumField(); i++ {
		field := t.Field(i)
		subPath := field.Tag.Get(pathTagName)

		if util.IsEmptyString(subPath) {
			continue
		}

		// get field related method according to the rule
		searchMethodName := field.Name + methodSuffix
		m := reflect.ValueOf(c).MethodByName(searchMethodName)

		fullPath := rootPath + subPath
		handler := toGinHandlerFunc(m)

		if strings.HasPrefix(searchMethodName, httpGetPrefix) {
			router.GET(fullPath, handler)
			continue
		}

		if strings.HasPrefix(searchMethodName, httpPostPrefix) {
			router.POST(fullPath, handler)
			continue
		}

		if strings.HasPrefix(searchMethodName, httpDeletePrefix) {
			router.DELETE(fullPath, handler)
			continue
		}
	}
}

func toGinHandlerFunc(method reflect.Value) gin.HandlerFunc {
	return (method.Interface()).(func(*gin.Context))
}
