package task

import (
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
	"github.com/pkg/errors"
)

const (
	TypeOrderCleanup = "order:cleanup"
)

type OrderTaskPayload struct {
	OrderID string
}

func NewCleanupOrderTask(payload OrderTaskPayload) (*asynq.Task, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal cleanup order task payload")
	}

	return asynq.NewTask(
		TypeOrderCleanup,
		bytes,
		asynq.TaskID(payload.OrderID),
		asynq.ProcessIn(time.Hour*24),
		asynq.MaxRetry(3),
	), nil
}
