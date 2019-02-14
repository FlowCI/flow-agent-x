package dao

import "time"

type MockSubEntity struct {
	ID        string `db:"column=id,pk=true,nullable=false"`
	Name      string `db:"column=name"`
	Age       int    `db:"column=age"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
