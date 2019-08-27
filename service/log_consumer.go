package service

import (
	"bytes"
	"encoding/json"
	"flow-agent-x/executor"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"flow-agent-x/config"
	"flow-agent-x/domain"
	"flow-agent-x/util"

	"github.com/streadway/amqp"
)

// Push stdout, stderr log back to server
func logConsumer(executor *executor.ShellExecutor) {
	config := config.GetInstance()
	cmd := executor.CmdIn
	logChannel := executor.GetLogChannel()
	rawChannel := executor.GetRawChannel()

	// upload log after flush!!
	defer func() {
		err := uploadLog(executor.Path.Raw)
		util.LogIfError(err)

		err = uploadLog(executor.Path.Log)
		util.LogIfError(err)

		util.LogDebug("[Exit]: logConsumer")
	}()

	go func() {
		for {
			_, ok := <-logChannel
			if !ok {
				break
			}
		}
	}()

	for {
		raw, ok := <-rawChannel
		if !ok {
			break
		}

		util.LogDebug("[Raw]: %s", raw)

		if config.HasQueue() {
			exchangeName := config.Settings.Queue.LogsExchange
			channel := config.Queue.LogChannel

			logItem := &domain.LogItem{CmdID: cmd.ID, Content: raw}
			writeLogToQueue(exchangeName, channel, logItem)
		}
	}
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

func writeLogToQueue(exchange string, qChannel *amqp.Channel, item *domain.LogItem) {
	_ = qChannel.Publish(exchange, "", false, false, amqp.Publishing{
		ContentType: util.HttpTextPlain,
		Body:        []byte(item.String()),
	})
}
