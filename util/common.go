package util

import (
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	UnixNewLine       = "\n"
	UnixPathSeparator = "/"

	WinNewLine       = "\r\n"
	WinPathSeparator = "\\"

	LineBreak = '\n'
	EmptyStr  = ""

	OSWin   = "windows"
	OSLinux = "linux"
	OSMac   = "darwin"
)

var (
	HomeDir = ""
)

func init() {
	HomeDir, _ = os.UserHomeDir()
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

func IsByteStartWith(src []byte, start []byte) bool {
	if len(src) < len(start) {
		return false
	}

	for i, c := range start {
		if src[i] != c {
			return false
		}
	}

	return true
}

func BytesTrimRight(src []byte, trim []byte) []byte {
	if len(src) < len(trim) {
		return src
	}

	canTrim := true

	j := len(trim) - 1
	for i := len(src) - 1; i >= len(src)-len(trim); i-- {
		if src[i] != trim[j] {
			canTrim = false
			break
		}
		j--
	}

	if canTrim {
		return src[0 : len(src)-len(trim)]
	}

	return src
}

func TrimLeftString(src, trim string) string {
	if len(src) < len(trim) {
		return src
	}

	index := -1
	for i := 0; i < len(trim); i++ {
		if src[i] == trim[i] {
			index = i
			continue
		}
	}

	if index == -1 {
		return ""
	}

	return src[index+1:]
}

func UTF16BytesToString(b []byte, o binary.ByteOrder) string {
	utf := make([]uint16, (len(b)+(2-1))/2)

	for i := 0; i+(2-1) < len(b); i += 2 {
		utf[i/2] = o.Uint16(b[i:])
	}

	if len(b)/2 < len(utf) {
		utf[len(utf)-1] = utf8.RuneError
	}

	return string(utf16.Decode(utf))
}
