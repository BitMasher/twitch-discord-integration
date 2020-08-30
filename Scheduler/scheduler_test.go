package Scheduler

import (
	"context"
	"testing"
)

func TestSubscribeWebhooks(t *testing.T) {
	err := SubscribeWebhooks(context.Background(), PubSubMessage{Data:[]byte{0}})
	if err != nil {
		t.Error(err)
	}
}