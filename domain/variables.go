package domain

import "fmt"

// Variables applied for envrionment variable as key, value
type Variables map[string]string

// NilOrEmpty detect variable is nil or empty
func NilOrEmpty(v Variables) bool {
	return v == nil || v.IsEmpty()
}

// ToStringArray convert variables map to key=value string array
func (v Variables) ToStringArray() []string {
	array := make([]string, len(v))

	index := 0
	for key, val := range v {
		array[index] = fmt.Sprintf("%s=%s", key, val)
		index++
	}

	return array
}

// IsEmpty to check is empty varaibles
func (v Variables) IsEmpty() bool {
	return len(v) == 0
}
