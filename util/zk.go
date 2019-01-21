package util

import (
	"time"

	"github.com/samuel/go-zookeeper/zk"
)

const (
	ZkNodeTypePersistent = int32(0)
	ZkNodeTypeEphemeral  = int32(zk.FlagEphemeral)
)

type ZkClient struct {
	conn *zk.Conn
}

// Connect zookeeper host
func (client *ZkClient) Connect(host string) error {
	// make connection of zk
	conn, _, err := zk.Connect([]string{host}, time.Second)

	if err != nil {
		return err
	}

	client.conn = conn
	return nil
}

// Create create node with node type and data
func (client *ZkClient) Create(path string, nodeType int32, data string) (string, error) {
	acl := zk.WorldACL(zk.PermAll)
	return client.conn.Create(path, []byte(data), nodeType, acl)
}

// Exist check the node is exist
func (client *ZkClient) Exist(path string) (bool, error) {
	exist, _, err := client.conn.Exists(path)
	return exist, err
}

func (client *ZkClient) Data(path string) (string, error) {
	bytes, _, err := client.conn.Get(path)
	return string(bytes), err
}

func (client *ZkClient) Delete(path string) error {
	exist, _ := client.Exist(path)

	if !exist {
		return nil
	}

	return client.conn.Delete(path, 0)
}

// Close release connection
func (client *ZkClient) Close() {
	if client.conn != nil {
		client.conn.Close()
	}
}
