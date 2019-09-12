package dao

import (
	"database/sql"

	"github/flowci/flow-agent-x/util"

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
	if c.db != nil {
		c.db.Close()
	}
}

func (c *Client) Create(entity interface{}) error {
	builder := QueryBuilder{
		entity: entity,
	}

	sqlStmt, err := builder.create()
	if util.HasError(err) {
		return err
	}

	_, err = c.db.Exec(sqlStmt)
	return err
}
