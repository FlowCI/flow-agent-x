package service

import (
	"bufio"
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

func writeLogToQueue(exchange string, qChannel *amqp.Channel, item *domain.LogItem) {
	qChannel.Publish(exchange, "", false, false, amqp.Publishing{
		ContentType: util.HttpTextPlain,
		Body:        []byte(item.String()),
	})
}

func writeLogToFile(w *bufio.Writer, item *domain.LogItem) {
	w.WriteString(item.Content)
	w.WriteByte(util.UnixLineBreak)
}
