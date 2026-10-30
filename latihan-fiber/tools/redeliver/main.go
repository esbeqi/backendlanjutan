// Command redeliver enqueues multiple task copies for one outbox event.
// It is a development test tool; do not run against a production queue.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/hibiken/asynq"
	"latihan-fiber/app/model"
	"latihan-fiber/config"
	"latihan-fiber/database"
)

func main() {
	outboxID := flag.Int64("outbox", 0, "outbox event ID")
	copies := flag.Int("copies", 5, "number of copies to enqueue")
	flag.Parse()
	if *outboxID < 1 || *copies < 1 || *copies > 100 {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/redeliver -outbox <id> -copies <1..100>")
		os.Exit(2)
	}
	config.LoadEnv()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := database.NewPool(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "database connection failed")
		os.Exit(1)
	}
	defer pool.Close()
	var eventType string
	var payload []byte
	if err := pool.QueryRow(ctx, `SELECT event_type, payload FROM outbox WHERE id=$1`, *outboxID).Scan(&eventType, &payload); err != nil {
		fmt.Fprintln(os.Stderr, "outbox event not found")
		os.Exit(1)
	}
	job, err := json.Marshal(model.NotificationJob{OutboxID: *outboxID, EventType: eventType, Payload: payload})
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not encode notification task")
		os.Exit(1)
	}
	client := asynq.NewClient(config.RedisOpt())
	defer client.Close()
	for i := 1; i <= *copies; i++ {
		task := asynq.NewTask(model.TaskSendNotification, job,
			asynq.TaskID(fmt.Sprintf("redeliver-outbox-%d-copy-%d", *outboxID, i)),
			asynq.Queue(model.QueueNotifications), asynq.MaxRetry(5),
			asynq.Timeout(30*time.Second), asynq.Retention(24*time.Hour))
		if _, err := client.EnqueueContext(ctx, task); err != nil {
			fmt.Fprintf(os.Stderr, "enqueue copy %d failed\n", i)
			os.Exit(1)
		}
	}
	fmt.Printf("enqueued %d copies for outbox %d\n", *copies, *outboxID)
}
