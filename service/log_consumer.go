package service

import (
	"bufio"
	"bytes"
	"encoding/json"
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
func logConsumer(cmd *domain.CmdIn, channel <-chan *domain.LogItem) {
	defer util.LogDebug("Exit: log consumer")

	config := config.GetInstance()
	logFilePath := filepath.Join(config.LoggingDir, cmd.ID+".log")

	f, _ := os.Create(logFilePath)
	defer f.Close()

	writer := bufio.NewWriter(f)

	// upload log after flush!!
	defer func() {
		writer.Flush()

		err := uploadLog(cmd)
		util.LogIfError(err)
	}()

	for {
		item, ok := <-channel
		if !ok {
			break
		}

		util.LogDebug(item.Content)

		writeLogToFile(writer, item)

		if config.HasQueue() {
			exchangeName := config.Settings.LogsExchangeName
			channel := config.Queue.LogChannel
			writeLogToQueue(exchangeName, channel, item)
		}
	}
}

func uploadLog(cmd *domain.CmdIn) error {
	config := config.GetInstance()
	logFile := filepath.Join(config.LoggingDir, cmd.ID+".log")

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

	written, err := io.Copy(part, file)
	if util.HasError(err) {
		return err
	}

	// flush file to writer
	writer.Close()

	util.LogDebug("Buffer length %d: %d", len(body.Bytes()), written)

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
		util.LogDebug("Log for cmd '%s' has been uploaded", cmd.ID)
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

func writeLogToFile(w *bufio.Writer, item *domain.LogItem) {
	_, _ = w.WriteString(item.Content)
	_ = w.WriteByte(util.UnixLineBreak)
}
