package service

import (
	"bufio"
	"bytes"
	"io"
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
	defer writer.Flush()

	for {
		item, ok := <-channel
		if !ok {
			return
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

func uploadLog(cmd *domain.ExecutedCmd) error {
	config := config.GetInstance()
	logFile := filepath.Join(config.LoggingDir, cmd.ID+".log")

	// read file
	file, err := os.Open(logFile)
	if err != nil {
		return err
	}

	defer file.Close()

	// construct multi part
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	defer func() {
		_ = writer.Close()
	}()

	part, err := writer.CreateFormFile("file", filepath.Base(logFile))
	if err != nil {
		return err
	}

	_, err = io.Copy(part, file)

	// send request
	url := config.Server + "/agents/logs/upload"
	request, _ := http.NewRequest("POST", url, body)
	request.Header.Set(util.HttpHeaderAgentToken, config.Token)
	request.Header.Set(util.HttpHeaderContentType, writer.FormDataContentType())

	http.DefaultClient.Do(request)
	//TODO: test
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
