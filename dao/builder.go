package dao

import (
	"fmt"
	"reflect"
	"strings"

	u "github.com/flowci/flow-agent-x/util"
)

type QueryBuilder struct {
	entity interface{}

	table   string
	columns []*EntityColumn
}

// init querybuilder with metadata
func initQueryBuilder(entity interface{}) *QueryBuilder {
	builder := new(QueryBuilder)

	t := u.GetType(entity)
	builder.entity = entity
	builder.table = flatCamelString(t.Name())
	builder.columns = make([]*EntityColumn, t.NumField())

	numOfNil := 0
	value := u.GetValue(entity)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		val := field.Tag.Get(tag)

		column := parseEntityColumn(val)
		if column == nil {
			numOfNil++
			continue
		}

		column.Value = value.Field(i)
		column.Type = field.Type.Kind()

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

func (builder *QueryBuilder) insert() (string, error) {
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

	for i, c := range builder.columns {
		query, err := toString(c.Value)
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
