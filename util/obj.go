package util

import (
	"log"
	"reflect"
)

func PanicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}

func RecoverPanic(handler func(e error)) {
	if r := recover(); r != nil {
		if handler != nil {
			handler(r.(error))
		}
	}
}

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

func HasString(s string) bool {
	return s != ""
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

func GetValue(v interface{}) reflect.Value {
	val := reflect.ValueOf(v)

	if val.Kind() == reflect.Ptr {
		return val.Elem()
	}

	return val
}
