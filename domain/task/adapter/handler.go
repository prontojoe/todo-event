package adapter

import (
	"context"
	"errors"
	"log/slog"

	"todoe/domain/task/domain"
	"todoe/domain/task/port"
	"todoe/internal/event"
)

func NewSaveHandler(repo port.Repository) func(context.Context, event.Event) error {
	return func(ctx context.Context, e event.Event) error {
		task, ok := e.Payload.(domain.Task)
		if !ok {
			slog.Error("handler: invalid payload type", "expected", "domain.Task", "got", e.Payload)
			return errors.New("invalid payload type")
		}
		result := repo.Save(ctx, task)
		if result.IsError() {
			return result.Error()
		}
		slog.Info("handler: task saved", "event_type", e.Type)
		return nil
	}
}
