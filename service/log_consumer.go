package service

import (
	"bufio"
	"os"
	"path/filepath"

	"github.com/flowci/flow-agent-x/config"
	"github.com/flowci/flow-agent-x/domain"
	"github.com/flowci/flow-agent-x/executor"
	"github.com/flowci/flow-agent-x/util"
	"github.com/streadway/amqp"
)

// Push stdout, stderr log back to server
func logConsumer(cmd *domain.CmdIn, channel executor.LogChannel) {
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
			channel := config.Queue.Channel
			writeLogToQueue(exchangeName, channel, item)
		}
	}
}

func writeLogToQueue(exchange string, qChannel *amqp.Channel, item *domain.LogItem) {
	msg := amqp.Publishing{
		ContentType: util.HttpTextPlain,
		Body:        []byte(item.String()),
	}

	qChannel.Publish(exchange, "", false, false, msg)
}

func writeLogToFile(w *bufio.Writer, item *domain.LogItem) {
	w.WriteString(item.Content)
	w.WriteByte(util.UnixLineBreak)
}
