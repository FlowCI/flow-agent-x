package service

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/proto"
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

		writer.Write(log.Raw)
		util.LogDebug("[LOG]: %s", log)

		// send to queue
		if config.HasQueue() {
			exchange := config.Settings.Queue.LogsExchange
			channel := config.Queue.LogChannel

			item, err := proto.Marshal(&domain.LogItem{
				CmdId:        log.CmdId,
				ContentInB64: base64.StdEncoding.EncodeToString(log.Raw),
			})

			if err != nil {
				continue
			}

			_ = channel.Publish(exchange, "", false, false, amqp.Publishing{
				ContentType: util.HttpProtobuf,
				Body:        item,
			})
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
