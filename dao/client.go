package dao

import (
	"database/sql"

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

	sqlStmt := `
	create table foo (id integer not null primary key, name text);
	delete from foo;
	`
	_, err := c.db.Exec(sqlStmt)
	return err
}
