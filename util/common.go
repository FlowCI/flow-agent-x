package util

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
)

const (
	UnixLineBreak    = '\n'
	UnixLineBreakStr = "\n"
)

func HasError(err error) bool {
	return err != nil
}

// FailOnError exit program with err
func FailOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

// IsEmptyString to check input s is empty
func IsEmptyString(s string) bool {
	return s == ""
}

// IsPointerType to check the input v is pointer type
func IsPointerType(v interface{}) bool {
	return reflect.ValueOf(v).Kind() == reflect.Ptr
}

// GetType get type of pointer
func GetType(v interface{}) reflect.Type {
	if IsPointerType(v) {
		val := reflect.ValueOf(v)
		return val.Elem().Type()
	}

	return reflect.TypeOf(v)
}

// ParseString parse string which include system env variable
func ParseString(src string) string {
	if IsEmptyString(src) {
		return src
	}

	for i := 0; i < len(src); i++ {
		if src[i] != '$' {
			continue
		}

		// left bracket index
		lIndex := i + 1
		if src[lIndex] != '{' {
			continue
		}

		// find right bracket index
		for rIndex := lIndex + 1; rIndex < len(src); rIndex++ {
			if src[rIndex] != '}' {
				continue
			}

			env := src[lIndex+1 : rIndex]
			val := os.Getenv(env)

			src = strings.Replace(src, fmt.Sprintf("${%s}", env), val, -1)
			i = rIndex
			break
		}
	}

	return src
}
