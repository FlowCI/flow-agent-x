package dao

import (
	"strings"

	"github.com/flowci/flow-agent-x/util"
)

func toCreateQuery(e interface{}) (string, error) {
	t := util.GetType(e)
	tableName := FlatCamelString(t.Name())

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
