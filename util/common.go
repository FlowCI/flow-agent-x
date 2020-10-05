package util

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

const (
	LineBreak = '\n'
	EmptyStr = ""

	OSWin   = "windows"
	OSLinux = "linux"
	OSMac   = "darwin"
)

var (
	NewLine = "\n"
)

func init() {
	if IsWindows() {
		NewLine = "\r\n"
	}
}

func OS() string {
	if IsMac() {
		return "MAC"
	}

	if IsWindows() {
		return "WIN"
	}

	if IsLinux() {
		return "LINUX"
	}

	return "UNKNOWN"
}

func IsMac() bool {
	return runtime.GOOS == OSMac
}

func IsLinux() bool {
	return runtime.GOOS == OSLinux
}

func IsWindows() bool {
	return runtime.GOOS == OSWin
}

func PointerBoolean(val bool) *bool {
	p := val
	return &p
}

func IndexOfFirstSpace(str string) int {
	for i := 0; i < len(str); i++ {
		if i == ' ' {
			return i
		}
	}
	return -1
}

// ParseString parse string which include system env variable
func ParseString(src string) string {
	return parseVariablesFrom(src, os.Getenv)
}

func ParseStringWithSource(src string, source map[string]string) string {
	return parseVariablesFrom(src, func(env string) string {
		return source[env]
	})
}

// replace ${VAR} with actual variable value
func parseVariablesFrom(src string, getVariable func(string) string) string {
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
			val := getVariable(env)

			// do not replace if no value found
			if IsEmptyString(val) {
				break
			}

			src = strings.Replace(src, fmt.Sprintf("${%s}", env), val, -1)
			i = rIndex
			break
		}
	}

	return src
}

func GetEnv(env, def string) string {
	val, ok := os.LookupEnv(env)
	if ok {
		return val
	}
	return def
}

func ByteToMB(bytes uint64) uint64 {
	return (bytes / 1024) / 1024
}
