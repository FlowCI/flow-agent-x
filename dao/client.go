package dao

import (
	"database/sql"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"

	"github.com/flowci/flow-agent-x/util"
	_ "github.com/mattn/go-sqlite3"
)

const (
	file = "agent.db"
)

// Client for sqlite3
type Client struct {
	dbPath string
	db     *sql.DB
}

func NewInstance(path string) (*Client, error) {
	instance := &Client{
		dbPath: path,
	}

	db, err := sql.Open("sqlite3", path)

	if util.HasError(err) {
		return nil, err
	}

	instance.db = db
	return instance, nil
}

func (c *Client) Close() {
	if !util.IsNil(c.db) {
		c.db.Close()
	}
}

func (c *Client) Create(entity interface{}) error {
	t := util.GetType(entity)

	tableName := FlatCamelString(t.Name())
	fmt.Println(tableName)

	super := initEntityValue(entity)
	fmt.Println(super)

	for i := 1; i < t.NumField(); i++ {
		field := t.Field(i)
		val, ok := field.Tag.Lookup(tag)

		if !ok {
			continue
		}

		fmt.Println(val)
	}

	sqlStmt := `
	create table foo (id integer not null primary key, name text);
	delete from foo;
	`
	_, err := c.db.Exec(sqlStmt)
	return err
}

func isEntityInherit(v interface{}) bool {
	t := util.GetType(v)
	return t.Field(0).Name == "Entity"
}

func initEntityValue(v interface{}) reflect.Value {
	super := reflect.ValueOf(v).Elem().Field(0)

	id := super.Field(0)
	if util.IsEmptyString(id.String()) {
		id.SetString(uuid.New().String())
	}

	createdAt := super.Field(1)
	if createdAt.Interface().(time.Time).IsZero() {
		createdAt.Set(reflect.ValueOf(time.Now()))
	}

	updatedAt := super.Field(2)
	if updatedAt.Interface().(time.Time).IsZero() {
		updatedAt.Set(reflect.ValueOf(time.Now()))
	}

	return super
}
