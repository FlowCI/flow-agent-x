package service

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"github/flowci/flow-agent-x/executor"
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
		appConfig := config.GetInstance()

		defer func() {
			// upload log after flush!!
			_ = logFileWriter.Flush()
			_ = f.Close()

			err := appConfig.Client.UploadLog(logPath)
			util.LogIfError(err)
			util.LogDebug("[Exit]: LogConsumer")
		}()

		for b64Log := range executor.Stdout() {

			// write to file
			log, err := base64.StdEncoding.DecodeString(b64Log)
			if err == nil {
				logFileWriter.Write(log)
				util.LogDebug("[ShellLog]: %s", string(log))
			}

			if appConfig.HasQueue() {
				channel := appConfig.Queue.Channel
				exchange := appConfig.Settings.Queue.ShellLogEx

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
		appConfig := config.GetInstance()
		for b64Log := range executor.TtyOut() {
			if !appConfig.HasQueue() {
				continue
			}

			channel := appConfig.Queue.Channel
			exchange := appConfig.Settings.Queue.TtyLogEx

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
