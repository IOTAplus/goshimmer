package dashboard

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
)

var liveFeedWorkerCount = 1
var liveFeedWorkerQueueSize = 50
var liveFeedWorkerPool *workerpool.WorkerPool

func configureLiveFeed() {
	liveFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		task.Param(0).(*message.CachedMessage).Consume(func(message *message.Message) {
			broadcastWsMessage(&wsmsg{MsgTypeMessage, &msg{message.ID().String(), 0}})
		})

		task.Return(nil)
	}, workerpool.WorkerCount(liveFeedWorkerCount), workerpool.QueueSize(liveFeedWorkerQueueSize))
}

func runLiveFeed() {
	newMsgRateLimiter := time.NewTicker(time.Second / 10)
	notifyNewMsg := events.NewClosure(func(message *message.CachedMessage, metadata *tangle.CachedMessageMetadata) {
		metadata.Release()

		select {
		case <-newMsgRateLimiter.C:
			_, ok := liveFeedWorkerPool.TrySubmit(message)
			if !ok {
				message.Release()
			}
		default:
			message.Release()
		}
	})

	if err := daemon.BackgroundWorker("Dashboard[MsgUpdater]", func(shutdownSignal <-chan struct{}) {
		messagelayer.Tangle().Events.MessageAttached.Attach(notifyNewMsg)
		liveFeedWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Dashboard[MsgUpdater] ...")
		messagelayer.Tangle().Events.MessageAttached.Detach(notifyNewMsg)
		newMsgRateLimiter.Stop()
		liveFeedWorkerPool.Stop()
		log.Info("Stopping Dashboard[MsgUpdater] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
