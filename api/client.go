package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
)

const (
	timeout               = 30 * time.Second
	connStateConnected    = 1
	connStateReconnecting = 2
	bufferSize            = 64 * 1024
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

type (
	Client interface {
		Connect(*domain.AgentInit) error
		ReConn() <-chan struct{}

		UploadLog(filePath string) error
		ReportProfile(*domain.Resource) error

		GetCmdIn() <-chan []byte
		SendCmdOut(out domain.CmdOut) error
		SendShellLog(jobId, stepId, b64Log string)
		SendTtyLog(ttyId, b64Log string)

		Close()
	}

	client struct {
		token      string
		server     string
		client     *http.Client
		cmdInbound chan []byte
		pending    chan *message

		reConn    chan struct{}
		connState int32
		conn      *websocket.Conn
	}

	message struct {
		event string
		body  []byte
	}
)

func (c *client) Connect(init *domain.AgentInit) error {
	u, err := url.Parse(c.server)
	if err != nil {
		return err
	}

	u.Scheme = "ws"
	u.Path = "agent"

	header := http.Header{}
	header.Add(headerToken, c.token)

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: timeout,
		ReadBufferSize:   bufferSize,
		WriteBufferSize:  bufferSize,
	}

	// build connection
	c.conn, _, err = dialer.Dial(u.String(), header)
	if err != nil {
		return err
	}

	c.conn.SetReadLimit(bufferSize)
	c.setConnState(connStateConnected)

	// send init connect event
	_, err = c.sendMessageWithResp(eventConnect, init)
	if err != nil {
		return err
	}

	// start to read message
	go c.readMessage()
	go c.consumePendingMessage()

	util.LogInfo("Agent is connected to server %s", c.server)
	return nil
}

func (c *client) ReConn() <-chan struct{} {
	return c.reConn
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

func (c *client) GetCmdIn() <-chan []byte {
	return c.cmdInbound
}

func (c *client) SendCmdOut(out domain.CmdOut) error {
	_ = c.sendMessageWithBytes(eventCmdOut, out.ToBytes())
	util.LogDebug("Result of cmd been pushed")
	return nil
}

func (c *client) SendShellLog(jobId, stepId, b64Log string) {
	body := &domain.ShellLog{
		JobId:  jobId,
		StepId: stepId,
		Log:    b64Log,
	}

	_ = c.sendMessageWithJson(eventShellLog, body)
}

func (c *client) SendTtyLog(ttyId, b64Log string) {
	body := &domain.TtyLog{
		ID:  ttyId,
		Log: b64Log,
	}
	_ = c.sendMessageWithJson(eventTtyLog, body)
}

func (c *client) Close() {
	if c.conn != nil {
		close(c.pending)
		_ = c.conn.Close()
	}
}

func (c *client) setConnState(state int32) {
	atomic.StoreInt32(&c.connState, state)
}

func (c *client) isReConnecting() bool {
	return atomic.LoadInt32(&c.connState) == connStateReconnecting
}

func (c *client) isConnected() bool {
	return atomic.LoadInt32(&c.connState) == connStateConnected
}

func (c *client) readMessage() {
	// start receive data
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			util.LogWarn("err on read message: %s", err.Error())
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.setConnState(connStateReconnecting)
				c.reConn <- struct{}{}
				c.conn = nil
				break
			}

			panic(err)
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		c.cmdInbound <- message
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

func (c *client) consumePendingMessage() {
	for message := range c.pending {
		// wait until connected
		for !c.isConnected() {
			time.Sleep(5 * time.Second)
		}

		_ = c.conn.WriteMessage(websocket.BinaryMessage, buildMessage(message.event, message.body))
		util.LogInfo("pending message has been sent: %s", message.event)
	}
}

func (c *client) sendMessageWithJson(event string, msg interface{}) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.pending <- &message{
		event: event,
		body:  body,
	}
	return nil
}

func (c *client) sendMessageWithBytes(event string, body []byte) error {
	c.pending <- &message{
		event: event,
		body:  body,
	}
	return nil
}

func (c *client) sendMessageWithResp(event string, msg interface{}) (resp *domain.Response, out error) {
	defer func() {
		if r := recover(); r != nil {
			out = r.(error)
		}
	}()

	body, err := json.Marshal(msg)
	util.PanicIfErr(err)

	err = c.conn.WriteMessage(websocket.BinaryMessage, buildMessage(event, body))
	util.PanicIfErr(err)

	_, data, err := c.conn.ReadMessage()
	util.PanicIfErr(err)

	resp = &domain.Response{}
	err = json.Unmarshal(data, resp)
	util.PanicIfErr(err)

	if !resp.IsOk() {
		out = fmt.Errorf(resp.Message)
		return
	}

	return
}
