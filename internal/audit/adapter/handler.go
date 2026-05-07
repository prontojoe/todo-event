package adapter

import (
	"context"
	"time"

	"github.com/samber/mo"

	taskdomain "todoe/domain/task/domain"
	"todoe/internal/audit/domain"
	"todoe/internal/event"
)

type AuditRepo interface {
	Save(ctx context.Context, record domain.AuditRecord) mo.Result[struct{}]
}

type Handler struct {
	repo AuditRepo
}

func NewHandler(repo AuditRepo) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Handle(ctx context.Context, e event.Event) error {
	task, ok := e.Payload.(taskdomain.Task)
	if !ok {
		return nil
	}

	record := domain.AuditRecord{
		TaskID:    task.ID,
		EventType: e.Type,
		Status:    string(task.Status),
		Time:      time.Now(),
	}

	if res := h.repo.Save(ctx, record); res.IsError() {
		return res.Error()
	}
	return nil
}
