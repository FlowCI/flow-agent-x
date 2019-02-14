package dao

import "errors"

var (
	ErrorNotEntity              = errors.New("db: the instance is not entity")
	ErrorDBTypeNotAvailable     = errors.New("db: db type not available")
	ErrorPrimaryKeyCannotBeNull = errors.New("db: primary key cannot set to null")
)
