package util

import (
	"fmt"
	"time"

	"github.com/samuel/go-zookeeper/zk"
)

const (
	ZkNodeTypePersistent = int32(0)
	ZkNodeTypeEphemeral  = int32(zk.FlagEphemeral)

	connectTimeout    = 10 // seconds
	disconnectTimeout = 5  // seconds
	sessionTimeout    = 1  // seconds
)

type (
	ZkCallbacks struct {
		OnDisconnected func()
	}

	ZkClient struct {
		Callbacks *ZkCallbacks
		conn      *zk.Conn
	}
)

func NewZkClient() *ZkClient {
	return &ZkClient{
		Callbacks: &ZkCallbacks{},
	}
}

// Connect zookeeper host
func (client *ZkClient) Connect(host string) error {
	// make connection of zk
	conn, events, err := zk.Connect([]string{host}, sessionTimeout*time.Second)
	if err != nil {
		return err
	}

	if !waitForConnection(events, connectTimeout) {
		return fmt.Errorf("zk server connection failed")
	}

	client.conn = conn
	go client.handleZkEvents(events)
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

func (client *ZkClient) handleZkEvents(events <-chan zk.Event) {
	var disconnectTimer *time.Timer

	for event := range events {
		if event.State == zk.StateConnected {
			LogInfo("zk: re-connected")
			if disconnectTimer != nil {
				_ = disconnectTimer.Stop()
			}
		}

		if event.State == zk.StateDisconnected {
			LogDebug("zk: disconnected")
			disconnectTimer = time.NewTimer(disconnectTimeout * time.Second)

			// close zk conn and fire disconnect event after disconnect timeout
			go func() {
				<-disconnectTimer.C
				client.conn.Close()
				if client.Callbacks.OnDisconnected != nil {
					client.Callbacks.OnDisconnected()
				}
			}()
		}
	}
}

func waitForConnection(events <-chan zk.Event, seconds int) bool {
	for event := range events {
		if event.State == zk.StateConnected {
			return true
		}

		if seconds == 0 {
			break
		}

		time.Sleep(1 * time.Second)
		seconds = seconds - 1
	}

	return false
}
