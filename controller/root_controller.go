package controller

import (
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

// RootController supper controller type
type RootController struct {
	router *gin.Engine
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
