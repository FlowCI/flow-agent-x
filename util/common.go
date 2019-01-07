package util

import (
	"log"
	"reflect"
)

// FailOnError exit program with err
func FailOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
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
