package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github/flowci/flow-agent-x/executor"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/streadway/amqp"
	"github/flowci/flow-agent-x/config"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
)

var (
	logBuffer = bytes.NewBuffer(make([]byte, 1024 * 1024 * 1)) // 1mb buffer
)

// Push stdout, stderr log back to server
func logConsumer(executor executor.Executor, logDir string) {
	config := config.GetInstance()
	logChannel := executor.LogChannel()

	// init path for shell, log and raw log
	logPath := filepath.Join(logDir, executor.CmdId()+".log")
	f, _ := os.Create(logPath)
	writer := bufio.NewWriter(f)

	// upload log after flush!!
	defer func() {
		_ = writer.Flush()
		_ = f.Close()

		err := uploadLog(logPath)
		util.LogIfError(err)

		util.LogDebug("[Exit]: logConsumer")
	}()

	for {
		log, ok := <-logChannel
		if !ok {
			break
		}

		writer.Write(log.Content)
		util.LogDebug("[LOG]: %s", log.Content)

		pushLog(config, log)
	}
}

func pushLog(config *config.Manager, log *domain.LogItem) {
	if !config.HasQueue() {
		return
	}

	defer logBuffer.Reset()

	exchange := config.Settings.Queue.LogsExchange
	channel := config.Queue.LogChannel

	_ = channel.Publish(exchange, "", false, false, amqp.Publishing{
		ContentType: util.HttpProtobuf,
		Body:        log.Write(logBuffer),
	})
}

func uploadLog(logFile string) error {
	config := config.GetInstance()

	// read file
	file, err := os.Open(logFile)
	if util.HasError(err) {
		return err
	}
	defer file.Close()

	// construct multi part
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(logFile))
	if util.HasError(err) {
		return err
	}

	_, err = io.Copy(part, file)
	if util.HasError(err) {
		return err
	}

	// flush file to writer
	writer.Close()

	// send request
	url := config.Server + "/agents/logs/upload"
	request, _ := http.NewRequest("POST", url, body)
	request.Header.Set(util.HttpHeaderAgentToken, config.Token)
	request.Header.Set(util.HttpHeaderContentType, writer.FormDataContentType())

	response, err := http.DefaultClient.Do(request)
	if util.HasError(err) {
		return err
	}

	// get response data
	raw, _ := ioutil.ReadAll(response.Body)
	var message domain.Response
	err = json.Unmarshal(raw, &message)
	if util.HasError(err) {
		return err
	}

	if message.IsOk() {
		util.LogDebug("[Uploaded]: %s", logFile)
		return nil
	}

	return fmt.Errorf(message.Message)
}