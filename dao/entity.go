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
		reflect.Int:    "INTEGER",
		reflect.String: "TEXT",
	}
)

type EntityColumn struct {
	Column   string
	Type     reflect.Kind
	Nullable bool
	Pk       bool
	Value    reflect.Value
}

func (f *EntityColumn) toQuery() (string, error) {
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
		query.WriteString(" NOT NULL")
	}

	if f.Pk {
		query.WriteString(" PRIMARY KEY")
	}

	return query.String(), nil
}

func parseEntityColumn(val string) *EntityColumn {
	if util.IsEmptyString(val) {
		return nil
	}

	count := 0
	entityField := &EntityColumn{
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
		entityField.Value = reflect.ValueOf(entityField)

		fieldOfEntityField := fieldVal.FieldByName(capitalFirstChar(key))

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
