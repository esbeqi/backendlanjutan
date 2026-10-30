package config

import (
	"testing"

	"latihan-fiber/app/model"
)

func TestWorkerListensOnNotificationQueue(t *testing.T) {
	queues := workerQueues()
	if _, ok := queues[model.QueueNotifications]; !ok {
		t.Fatalf("worker must listen on queue %q; configured queues: %#v", model.QueueNotifications, queues)
	}
}
