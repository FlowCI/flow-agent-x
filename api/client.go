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
	"strings"
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

		CachePut(jobId, name, workspace string, paths []string)
		CacheGet(jobId, name string) *domain.JobCache
		CacheDownload(cacheId, workspace, file string)

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

	// part: multipart data
	part struct {
		key  string
		file string
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
	defer util.RecoverPanic(func(e error) {
		err = e
	})

	buffer, contentType := c.buildMultipartContent([]*part{
		{
			key:  "file",
			file: filePath,
		},
	})

	// send request
	raw, err := c.send("POST", "logs/upload", contentType, buffer)
	util.PanicIfErr(err)

	_, err = c.parseResponse(raw, &domain.Response{})
	util.PanicIfErr(err)

	util.LogInfo("[Uploaded]: %s", filePath)
	return
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

func (c *client) CachePut(jobId, key, workspace string, paths []string) {
	defer util.RecoverPanic(func(e error) {
		util.LogWarn(e.Error())
	})

	tempDir, err := ioutil.TempDir("", "agent_cache_")
	util.PanicIfErr(err)
	defer os.RemoveAll(tempDir)

	var parts []*part
	for _, path := range paths {
		if !util.IsFileExists(path) {
			util.LogWarn("the file %s not exist", path)
			continue
		}

		if !strings.HasPrefix(path, workspace) {
			util.LogWarn("the cache path must be under workspace")
			continue
		}

		cacheZipName := encodeCacheName(workspace, path)
		zipPath := tempDir + util.UnixPathSeparator + cacheZipName
		err = util.Zip(path, zipPath, util.UnixPathSeparator)

		if err != nil {
			util.LogWarn(err.Error())
			continue
		}

		parts = append(parts, &part{
			key:  "files",
			file: zipPath,
		})
	}

	buffer, contentType := c.buildMultipartContent(parts)

	raw, err := c.send("POST", fmt.Sprintf("cache/%s/%s", jobId, key), contentType, buffer)
	util.PanicIfErr(err)

	_, err = c.parseResponse(raw, &domain.Response{})
	util.PanicIfErr(err)

	util.LogInfo("[CachePut] %d/%d files cached in %s", len(parts), len(paths), key)
}

func (c *client) CacheGet(jobId, key string) *domain.JobCache {
	raw, err := c.send("GET", fmt.Sprintf("cache/%s/%s", jobId, key), "", nil)
	util.PanicIfErr(err)

	resp, err := c.parseResponse(raw, &domain.JobCacheResponse{})
	util.PanicIfErr(err)

	jobCache := resp.(*domain.JobCacheResponse)
	return jobCache.Data
}

func (c *client) CacheDownload(cacheId, workspace, file string) {
	defer util.RecoverPanic(func(e error) {
		util.LogWarn(e.Error())
	})

	tmpPath := fmt.Sprintf("%s/%s.tmp", workspace, file)
	tmpFile, err := os.Create(tmpPath)
	util.PanicIfErr(err)

	err = c.download(fmt.Sprintf("cache/%s?file=%s", cacheId, file), tmpFile)
	util.PanicIfErr(err)

	zippedFile := workspace + util.UnixPathSeparator + file
	err = os.Rename(tmpPath, zippedFile)
	util.PanicIfErr(err)

	defer os.RemoveAll(zippedFile)

	cacheFileName := decodeCacheName(file)
	dest := workspace + util.UnixPathSeparator + cacheFileName

	err = util.Unzip(zippedFile, dest)
	util.PanicIfErr(err)
}

func (c *client) Close() {
	if c.conn != nil {
		close(c.pending)
		_ = c.conn.Close()
	}
}

func (c *client) buildMultipartContent(parts []*part) (*bytes.Buffer, string) {
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	defer writer.Close()

	for _, part := range parts {
		file, err := os.Open(part.file)
		util.PanicIfErr(err)

		part, err := writer.CreateFormFile(part.key, filepath.Base(part.file))
		util.PanicIfErr(err)

		_, err = io.Copy(part, file)
		util.PanicIfErr(err)

		_ = file.Close()
	}

	return buffer, writer.FormDataContentType()
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
func (c *client) send(method, path, contentType string, body io.Reader) ([]byte, error) {
	url := fmt.Sprintf("%s/agents/api/%s", c.server, path)
	req, _ := http.NewRequest(method, url, body)

	req.Header.Set(util.HttpHeaderContentType, contentType)
	req.Header.Set(util.HttpHeaderAgentToken, c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (c *client) download(path string, dist io.Writer) error {
	url := fmt.Sprintf("%s/agents/api/%s", c.server, path)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set(util.HttpHeaderAgentToken, c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	_, err = io.Copy(dist, io.TeeReader(resp.Body, &CounterWrite{}))
	if err != nil {
		return err
	}

	return nil
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
	defer util.RecoverPanic(func(e error) {
		out = e
	})

	body, err := json.Marshal(msg)
	util.PanicIfErr(err)

	err = c.conn.WriteMessage(websocket.BinaryMessage, buildMessage(event, body))
	util.PanicIfErr(err)

	_, data, err := c.conn.ReadMessage()
	util.PanicIfErr(err)

	message, err := c.parseResponse(data, &domain.Response{})
	util.PanicIfErr(err)

	resp = message.(*domain.Response)
	return
}

func (c *client) parseResponse(body []byte, resp domain.ResponseMessage) (domain.ResponseMessage, error) {
	// get response data
	err := json.Unmarshal(body, resp)
	if err != nil {
		return nil, err
	}

	if resp.IsOk() {
		return resp, nil
	}

	return nil, fmt.Errorf(resp.GetMessage())
}
