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
		logPath := filepath.Join(logDir, executor.CmdIn().ID+".log")
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
		for b64Log := range executor.Stdout() {

			// write to file
			log, err := base64.StdEncoding.DecodeString(b64Log)
			if err == nil {
				logFileWriter.Write(log)
				util.LogDebug("[ShellLog]: %s", string(log))
			}

			if config.HasQueue() {
				channel := config.Queue.Channel
				exchange := config.Settings.Queue.ShellLogEx

				jobId := executor.CmdIn().JobId
				stepId := executor.CmdIn().ID

				logContent := &domain.CmdStdLog{
					ID:      stepId,
					Content: b64Log,
				}

				marshal, _ := json.Marshal(logContent)

				_ = channel.Publish(exchange, "", false, false, amqp.Publishing{
					Body: marshal,
					Headers: map[string]interface{}{
						"id":     jobId,
						"stepId": stepId,
					},
				})
			}
		}
	}

	consumeTtyLog := func() {
		config := config.GetInstance()
		for b64Log := range executor.TtyOut() {
			if !config.HasQueue() {
				continue
			}

			channel := config.Queue.Channel
			exchange := config.Settings.Queue.TtyLogEx

			_ = channel.Publish(exchange, "", false, false, amqp.Publishing{
				Body: []byte(b64Log),
				Headers: map[string]interface{}{
					"id": executor.TtyId(),
				},
			})
		}
	}

	go consumeShellLog()
	go consumeTtyLog()
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
