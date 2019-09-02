package dao

import (
	"fmt"
	"reflect"
	"strings"

	u "flow-agent-x/util"
)

type QueryBuilder struct {
	entity     interface{}
	entityType reflect.Type

	table   string
	columns []*EntityColumn
	key     *EntityColumn
}

// init querybuilder with metadata
func initQueryBuilder(entity interface{}) *QueryBuilder {
	t := u.GetType(entity)

	builder := new(QueryBuilder)
	builder.entityType = t
	builder.entity = entity
	builder.table = flatCamelString(t.Name())
	builder.columns = make([]*EntityColumn, t.NumField())

	numOfNil := 0

	for i := 0; i < t.NumField(); i++ {
		column := parseEntityColumn(t.Field(i))

		if column == nil {
			numOfNil++
			continue
		}

		if column.Pk {
			builder.key = column
		}

		builder.columns[i-numOfNil] = column
	}

	builder.columns = builder.columns[:numOfNil+1]
	return builder
}

// create table by entity
func (builder *QueryBuilder) create() (string, error) {
	var sql strings.Builder
	sql.WriteString("CREATE TABLE IF NOT EXISTS " + builder.table)
	sql.WriteString(" (")

	for i, c := range builder.columns {
		if c == nil {
			continue
		}

		// create sql for field
		q, err := c.toQuery()
		if u.HasError(err) {
			return "", err
		}

		if i > 0 {
			sql.WriteByte(',')
		}

		sql.WriteString(q)
	}

	sql.WriteString(");")
	return sql.String(), nil
}

func (builder *QueryBuilder) drop() (string, error) {
	return "DROP TABLE IF EXISTS " + builder.table + ";", nil
}

func (builder *QueryBuilder) insert(data interface{}) (string, error) {
	if !isSameType(builder.entityType, data) {
		return "", ErrorNotEntity
	}

	var sql strings.Builder
	sql.WriteString("INSERT INTO ")
	sql.WriteString(builder.table)
	sql.WriteString(" (")

	for i, c := range builder.columns {
		sql.WriteString("'" + c.Column + "'")

		if i < len(builder.columns)-1 {
			sql.WriteString(",")
		}
	}

	sql.WriteString(")")
	sql.WriteString(" VALUES ")
	sql.WriteString("(")

	value := u.GetValue(data)
	for i, c := range builder.columns {
		query, err := toString(value.FieldByName(c.Field.Name))
		if u.HasError(err) {
			return u.EmptyStr, err
		}

		sql.WriteString(query)

		if i < len(builder.columns)-1 {
			sql.WriteString(",")
		}
	}

	sql.WriteString(");")

	return sql.String(), nil
}

func (builder *QueryBuilder) find(id string) (string, error) {
	var sql strings.Builder
	sql.WriteString("SELECT ")

	for i, c := range builder.columns {
		sql.WriteString("'" + c.Column + "'")

		if i < len(builder.columns)-1 {
			sql.WriteString(",")
		}
	}

	sql.WriteString(" FROM " + builder.table)
	sql.WriteString(fmt.Sprintf(" WHERE %s='%s';", builder.key.Column, id))

	return sql.String(), nil
}

func isSameType(source reflect.Type, data interface{}) bool {
	t := u.GetType(data)
	return t == source
}

// from value to sql type
func toString(val reflect.Value) (string, error) {
	if val.Kind() == reflect.String {
		return "'" + val.String() + "'", nil
	}

	if val.Kind() == reflect.Bool {
		return fmt.Sprintf("%t", val.Bool()), nil
	}

	if val.Kind() == reflect.Int {
		return fmt.Sprintf("%d", val.Int()), nil
	}

	return u.EmptyStr, ErrorUnsupporttedDataType
}
