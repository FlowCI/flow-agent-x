package dao

import (
	"strings"

	"github.com/flowci/flow-agent-x/util"
)

type QueryBuilder struct {
	entity interface{}
}

func (builder *QueryBuilder) create() (string, error) {
	t := util.GetType(builder.entity)
	tableName := flatCamelString(t.Name())

	var sql strings.Builder
	sql.WriteString("create " + tableName)
	sql.WriteString("(")

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		val := field.Tag.Get(tag)

		entityField := parseEntityField(val)
		if entityField == nil {
			continue
		}

		entityField.Type = field.Type.Kind()

		// create sql for field
		q, err := entityField.toQuery()
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
