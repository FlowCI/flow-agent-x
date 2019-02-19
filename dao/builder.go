package dao

import (
	"strings"

	"github.com/flowci/flow-agent-x/util"
)

type QueryBuilder struct {
	entity interface{}

	table   string
	columns []*EntityColumn
}

// init querybuilder with metadata
func initQueryBuilder(entity interface{}) *QueryBuilder {
	builder := new(QueryBuilder)

	t := util.GetType(entity)
	builder.entity = entity
	builder.table = flatCamelString(t.Name())
	builder.columns = make([]*EntityColumn, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		val := field.Tag.Get(tag)

		column := parseEntityColumn(val)
		if column == nil {
			continue
		}

		column.Type = field.Type.Kind()

		builder.columns[i] = column
	}

	return builder
}

func (builder *QueryBuilder) create() (string, error) {
	var sql strings.Builder
	sql.WriteString("CREATE TABLE " + builder.table)
	sql.WriteString(" (")

	for i, c := range builder.columns {
		if c == nil {
			continue
		}

		// create sql for field
		q, err := c.toQuery()
		if util.HasError(err) {
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
