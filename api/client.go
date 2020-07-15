package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
)

type Client struct {
	token  string
	server string
	client *http.Client
}

func (c *Client) GetSettings(init *domain.AgentInit) (out *domain.Settings, err error) {
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

func (c *Client) ReportProfile(r *domain.Resource) (err error) {
	body, err := json.Marshal(r)
	if err != nil {
		return
	}

	_, err = c.send("POST", "profile", util.HttpMimeJson, bytes.NewBuffer(body))
	return
}

func (c *Client) UploadLog(filePath string) (err error) {
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
	raw, err := c.send("POST", "upload", writer.FormDataContentType(), body)
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

// method: GET/POST, path: {server}/agents/api/:path
func (c *Client) send(method, path, contentType string, body io.Reader) (out []byte, err error) {
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

func NewClient(token, server string) *Client {
	transport := &http.Transport{
		MaxIdleConns:    5,
		IdleConnTimeout: 30 * time.Second,
	}

	return &Client{
		token:  token,
		server: server,
		client: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
	}
}
