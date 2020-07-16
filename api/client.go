package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/streadway/amqp"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
)

type (
	Client interface {
		HasQueueSetup() bool
		SetQueue(*domain.RabbitMQConfig, string) error

		GetSettings(*domain.AgentInit) (*domain.Settings, error)
		UploadLog(filePath string) error
		ReportProfile(*domain.Resource) error

		GetCmdIn() (<-chan amqp.Delivery, error)
		SendCmdOut(out domain.CmdOut) error
		SendShellLog(jobId, stepId, b64Log string)
		SendTtyLog(ttyId, b64Log string)

		Close()
	}

	client struct {
		token  string
		server string
		client *http.Client

		qLock    sync.Mutex
		qConn    *amqp.Connection
		qChannel *amqp.Channel
		qAgent   *amqp.Queue

		qCallback  string
		exShellLog string
		exTtyLog   string

		qAgentConsumer <-chan amqp.Delivery
	}
)

func (c *client) HasQueueSetup() bool {
	c.qLock.Lock()
	defer c.qLock.Unlock()
	return c.qConn != nil && c.qChannel != nil && c.qAgent != nil
}

func (c *client) SetQueue(config *domain.RabbitMQConfig, agentQ string) (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
		}
	}()

	c.qLock.Lock()
	defer c.qLock.Unlock()

	conn, err := amqp.Dial(config.Uri)
	util.PanicIfErr(err)

	ch, err := conn.Channel()
	util.PanicIfErr(err)

	// init agent job queue to receive job
	queue, err := ch.QueueDeclare(agentQ, false, false, false, false, nil)
	util.PanicIfErr(err)

	c.qConn = conn
	c.qChannel = ch
	c.qAgent = &queue

	c.qCallback = config.Callback
	c.exShellLog = config.ShellLogEx
	c.exTtyLog = config.TtyLogEx

	return
}

func (c *client) GetSettings(init *domain.AgentInit) (out *domain.Settings, err error) {
	defer func() {
		if err := recover(); err != nil {
			return
		}
	}()

	body, err := json.Marshal(init)
	util.PanicIfErr(err)

	raw, err := c.send("POST", "connect", util.HttpMimeJson, bytes.NewBuffer(body))
	util.PanicIfErr(err)

	var msg domain.SettingsResponse
	errFromJSON := json.Unmarshal(raw, &msg)
	util.PanicIfErr(errFromJSON)

	if msg.IsOk() {
		return msg.Data, nil
	}

	return nil, fmt.Errorf(msg.Message)
}

func (c *client) ReportProfile(r *domain.Resource) (err error) {
	body, err := json.Marshal(r)
	if err != nil {
		return
	}

	_, err = c.send("POST", "profile", util.HttpMimeJson, bytes.NewBuffer(body))
	return
}

func (c *client) UploadLog(filePath string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	file, err := os.Open(filePath)
	util.PanicIfErr(err)

	defer file.Close()

	// construct multi part
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	util.PanicIfErr(err)

	_, err = io.Copy(part, file)
	util.PanicIfErr(err)

	// flush file to writer
	_ = writer.Close()

	// send request
	raw, err := c.send("POST", "logs/upload", writer.FormDataContentType(), body)
	util.PanicIfErr(err)

	// get response data
	var message domain.Response
	err = json.Unmarshal(raw, &message)
	util.PanicIfErr(err)

	if message.IsOk() {
		util.LogInfo("[Uploaded]: %s", filePath)
		return nil
	}

	return fmt.Errorf(message.Message)
}

func (c *client) GetCmdIn() (<-chan amqp.Delivery, error) {
	if c.qAgentConsumer != nil {
		return c.qAgentConsumer, nil
	}

	msgs, err := c.qChannel.Consume(c.qAgent.Name, "", true, false, false, false, nil)

	if err != nil {
		return nil, err
	}

	c.qAgentConsumer = msgs
	return c.qAgentConsumer, nil
}

func (c *client) SendCmdOut(out domain.CmdOut) error {
	err := c.qChannel.Publish("", c.qCallback, false, false, amqp.Publishing{
		ContentType: util.HttpMimeJson,
		Body:        out.ToBytes(),
	})

	if err != nil {
		return err
	}

	util.LogDebug("Result of cmd been pushed")
	return nil
}

func (c *client) SendShellLog(jobId, stepId, b64Log string) {
	raw, err := json.Marshal(&domain.CmdStdLog{
		ID:      stepId,
		Content: b64Log,
	})

	if err != nil {
		util.LogWarn(err.Error())
		return
	}

	_ = c.qChannel.Publish(c.exShellLog, "", false, false, amqp.Publishing{
		Body: raw,
		Headers: map[string]interface{}{
			"id":     jobId,
			"stepId": stepId,
		},
	})
}

func (c *client) SendTtyLog(ttyId, b64Log string) {
	_ = c.qChannel.Publish(c.exTtyLog, "", false, false, amqp.Publishing{
		Body: []byte(b64Log),
		Headers: map[string]interface{}{
			"id": ttyId,
		},
	})
}

func (c *client) Close() {
	if c.HasQueueSetup() {
		_ = c.qChannel.Close()
		_ = c.qConn.Close()
	}
}

// method: GET/POST, path: {server}/agents/api/:path
func (c *client) send(method, path, contentType string, body io.Reader) (out []byte, err error) {
	url := fmt.Sprintf("%s/agents/api/%s", c.server, path)
	req, _ := http.NewRequest(method, url, body)

	req.Header.Set(util.HttpHeaderContentType, contentType)
	req.Header.Set(util.HttpHeaderAgentToken, c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	out, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	return
}

func NewClient(token, server string) Client {
	transport := &http.Transport{
		MaxIdleConns:    5,
		IdleConnTimeout: 30 * time.Second,
	}

	return &client{
		token:  token,
		server: server,
		client: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
	}
}
