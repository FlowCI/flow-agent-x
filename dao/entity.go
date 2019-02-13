package dao

import (
	"strings"
	"time"
	"unicode"
)

// Entity the base model
type Entity struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// FlatCamelString change camel string to string with '_'
func FlatCamelString(v string) string {
	var builder strings.Builder
	builder.Grow(len(v) + 5)

	for i, c := range v {
		r := rune(c)

		if unicode.IsUpper(r) {
			r = unicode.ToLower(r)

			if i > 0 {
				builder.WriteByte('_')
			}
		}

		builder.WriteByte(byte(r))
	}

	return builder.String()
}
