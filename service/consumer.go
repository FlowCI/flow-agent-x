package service

import (
	"bufio"
	"bytes"
	"encoding/base64"
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
func startLogConsumer(executor executor.Executor, logDir string) {
	// init path for shell, log and raw log
	consumeShellLog := func() {
		logPath := filepath.Join(logDir, executor.CmdId()+".log")
		f, _ := os.Create(logPath)
		logFileWriter := bufio.NewWriter(f)

		defer func() {
			// upload log after flush!!
			_ = logFileWriter.Flush()
			_ = f.Close()
			err := uploadLog(logPath)
			util.LogIfError(err)

			util.LogDebug("[Exit]: tLogConsumer")
		}()

		config := config.GetInstance()
		for log := range executor.LogChannel() {
			// write to file
			logFileWriter.Write(log)
			util.LogDebug("[ShellLog]: %s", string(log))

			if config.HasQueue() {
				channel := config.Queue.Channel
				exchange := config.Settings.Queue.ShellLogEx

				cmdLog := &domain.CmdLog{
					ID:      executor.CmdId(),
					Content: base64.StdEncoding.EncodeToString(log),
				}
				pushLog(channel, exchange, cmdLog)
			}
		}
	}

	consumeTtyLog := func() {
		config := config.GetInstance()
		for log := range executor.OutputStream() {
			util.LogDebug("[TtyLog]: %s", string(log))

			if config.HasQueue() {
				channel := config.Queue.Channel
				exchange := config.Settings.Queue.TtyLogEx

				cmdLog := &domain.CmdLog{
					ID:      executor.TtyId(),
					Content: base64.StdEncoding.EncodeToString(log),
				}
				pushLog(channel, exchange, cmdLog)
			}
		}
	}

	go consumeShellLog()
	go consumeTtyLog()
}

func pushLog(c *amqp.Channel, exchange string, log *domain.CmdLog) {
	raw, err := json.Marshal(log)
	if err != nil {
		util.LogWarn("Cannot marshal CmdLog to json")
		return
	}

	_ = c.Publish(exchange, "", false, false, amqp.Publishing{
		ContentType: util.HttpProtobuf,
		Body:        raw,
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
