package dao

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/flowci/flow-agent-x/util"
)

const (
	tag          = "db"
	tagSeparator = ","
	valSeparator = "="

	keyFieldColumn   = "column"
	keyFieldNullable = "nullable"
)

var (
	typeMapping = map[reflect.Kind]string{
		reflect.Int:    "integer",
		reflect.String: "text",
	}
)

type EntityField struct {
	Column   string
	Type     reflect.Kind
	Nullable bool
	Pk       bool
}

func (f *EntityField) toQuery() (string, error) {
	t := typeMapping[f.Type]

	if util.IsNil(t) {
		return util.EmptyStr, ErrorDBTypeNotAvailable
	}

	if f.Pk && f.Nullable {
		return util.EmptyStr, ErrorPrimaryKeyCannotBeNull
	}

	var query strings.Builder
	query.Grow(30)
	query.WriteString(f.Column)
	query.WriteByte(' ')
	query.WriteString(t)

	if !f.Nullable {
		query.WriteString(" not null")
	}

	if f.Pk {
		query.WriteString(" primary key")
	}

	return query.String(), nil
}

func parseEntityField(val string) *EntityField {
	if util.IsEmptyString(val) {
		return nil
	}

	count := 0
	entityField := &EntityField{
		Nullable: true,
		Pk:       false,
	}

	items := strings.Split(val, tagSeparator)

	for _, item := range items {
		kv := strings.Split(item, valSeparator)

		if len(kv) != 2 {
			continue
		}

		key := kv[0]
		val := kv[1]
		count++

		fieldVal := reflect.ValueOf(entityField).Elem()
		fieldOfEntityField := fieldVal.FieldByName(CapitalFirstChar(key))

		if fieldOfEntityField.Type().Kind() == reflect.String {
			fieldOfEntityField.SetString(val)
		}

		if fieldOfEntityField.Type().Kind() == reflect.Bool {
			b, _ := strconv.ParseBool(val)
			fieldOfEntityField.SetBool(b)
		}
	}

	// no valid entity field
	if count == 0 {
		return nil
	}

	return entityField
}
