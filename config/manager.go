package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/flowci/flow-agent-x/domain"
)

const (
	errSettingConnectFail = "Cannot get settings from server"
)

var (
	singleton *Manager
	once      sync.Once
)

// Manager to handle server connection and config
type Manager struct {
}

// GetInstance get singleton of config manager
func GetInstance() *Manager {
	once.Do(func() {
		singleton = &Manager{}
	})
	return singleton
}

// Connect get settings from server
func (m *Manager) Connect() (domain.Settings, error) {
	server, token, port := getVaraibles()

	uri := server + "/agents/connect"
	body, _ := json.Marshal(domain.AgentConnect{Token: token, Port: port})

	var message domain.SettingsResponse
	resp, errFromReq := http.Post(uri, "application/json", bytes.NewBuffer(body))
	if errFromReq != nil {
		return message.Data, fmt.Errorf(errSettingConnectFail)
	}

	defer resp.Body.Close()
	raw, _ := ioutil.ReadAll(resp.Body)
	errFromJSON := json.Unmarshal(raw, &message)

	if errFromJSON != nil {
		return message.Data, errFromJSON
	}

	if !message.IsOk() {
		return message.Data, fmt.Errorf(message.Message)
	}

	return message.Data, nil
}

func getVaraibles() (server string, token string, port int) {
	server = os.Getenv("FLOWCI_SERVER_URL")
	token = os.Getenv("FLOWCI_AGENT_TOKEN")
	port, _ = strconv.Atoi(os.Getenv("FLOWCI_AGENT_PORT"))
	return
}
