package dao

import (
	"reflect"
	"strings"
	"time"
)

const (
	tag          = "db"
	tagSeparator = ","
	valSeparator = "="

	keyFieldColumn = "column"
)

// Entity the base model
type Entity struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type EntityField struct {
	Column string
}

func ParseEntityField(val string) *EntityField {
	field := &EntityField{}

	items := strings.Split(val, tagSeparator)
	for _, item := range items {
		kv := strings.Split(item, valSeparator)

		if len(kv) != 2 {
			continue
		}

		key := kv[0]
		val := kv[1]

		fieldVal := reflect.ValueOf(field).Elem()
		fieldVal.FieldByName(CapitalFirstChar(key)).SetString(val)
	}

	return field
}
