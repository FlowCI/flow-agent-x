package dao

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestShouldBuildQueryForCreateTable(t *testing.T) {
	assert := assert.New(t)

	builder := initQueryBuilder(MockSubEntity{})
	query, err := builder.create()
	assert.Nil(err)

	expected := "CREATE TABLE IF NOT EXISTS mock_sub_entity (id TEXT NOT NULL PRIMARY KEY,name TEXT,age INTEGER);"
	assert.Equal(expected, query)
}

func TestShouldBuildQueryForDropTable(t *testing.T) {
	assert := assert.New(t)

	builder := initQueryBuilder(MockSubEntity{})
	query, _ := builder.drop()

	expected := "DROP TABLE IF EXISTS mock_sub_entity;"
	assert.Equal(expected, query)
}

func TestShouldBuildQueryForInsert(t *testing.T) {
	assert := assert.New(t)

	entity := &MockSubEntity{
		ID:        "12345",
		Name:      "yang",
		Age:       18,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	builder := initQueryBuilder(MockSubEntity{})
	query, _ := builder.insert(entity)

	expected := "INSERT INTO mock_sub_entity ('id','name','age') VALUES ('12345','yang',18);"
	assert.Equal(expected, query)
}

func TestShouldBuildQueryForFindByID(t *testing.T) {
	assert := assert.New(t)

	builder := initQueryBuilder(MockSubEntity{})
	query, _ := builder.find("12345")

	expected := "SELECT 'id','name','age' FROM mock_sub_entity WHERE id='12345';"
	assert.Equal(expected, query)
}
