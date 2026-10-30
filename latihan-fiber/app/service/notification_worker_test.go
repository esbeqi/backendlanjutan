package service

import (
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"latihan-fiber/helper"
)

func TestPermanentDeliveryIsMarkedSkipRetry(t *testing.T) {
	err := permanentDeliveryTaskError(helper.ErrPermanentDelivery)
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("permanent delivery error must wrap asynq.SkipRetry, got %v", err)
	}
}
