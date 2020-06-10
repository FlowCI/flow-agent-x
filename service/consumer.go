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

// Push stdout, stderr log back to server
func logConsumer(executor executor.Executor, logDir string) {
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

	consumeShellLog := func() {
		config := config.GetInstance()
		buffer := bytes.NewBuffer(make([]byte, 1024*1024*1)) // 1mb buffer
		for {
			log, ok := <-executor.LogChannel()
			if !ok {
				return
			}
			// write to file
			writer.Write(log.Content)
			util.LogDebug("[ShellLog]: %s", log.Content)
			if config.HasQueue() {
				channel := config.Queue.Channel
				exchange := config.Settings.Queue.ShellLogEx
				pushLog(channel, exchange, buffer, log)
			}
		}
	}

	consumeTtyLog := func() {
		config := config.GetInstance()
		buffer := bytes.NewBuffer(make([]byte, 1024*1024*1)) // 1mb buffer
		for {
			log, ok := <-executor.OutputStream()
			if !ok {
				return
			}
			util.LogDebug("[TtyLog]: %s", log.Content)
			if config.HasQueue() {
				channel := config.Queue.Channel
				exchange := config.Settings.Queue.TtyLogEx
				pushLog(channel, exchange, buffer, log)
			}
		}
	}

	go consumeShellLog()
	go consumeTtyLog()
}

func pushLog(c *amqp.Channel, exchange string, buffer *bytes.Buffer, log domain.CmdLog) {
	defer buffer.Reset()
	_ = c.Publish(exchange, "", false, false, amqp.Publishing{
		ContentType: util.HttpProtobuf,
		Body:        log.ToBytes(buffer),
	})
}

func uploadLog(logFile string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	config := config.GetInstance()

	// read file
	file, err := os.Open(logFile)
	util.PanicIfErr(err)

	defer file.Close()

	// construct multi part
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(logFile))
	util.PanicIfErr(err)

	_, err = io.Copy(part, file)
	util.PanicIfErr(err)

	// flush file to writer
	writer.Close()

	// send request
	url := config.Server + "/agents/logs/upload"
	request, _ := http.NewRequest("POST", url, body)
	request.Header.Set(util.HttpHeaderAgentToken, config.Token)
	request.Header.Set(util.HttpHeaderContentType, writer.FormDataContentType())

	response, err := http.DefaultClient.Do(request)
	util.PanicIfErr(err)

	// get response data
	raw, _ := ioutil.ReadAll(response.Body)
	var message domain.Response
	err = json.Unmarshal(raw, &message)
	util.PanicIfErr(err)

	if message.IsOk() {
		util.LogDebug("[Uploaded]: %s", logFile)
		return nil
	}

	return fmt.Errorf(message.Message)
}
