package adapter

import (
	"context"
	"errors"
	"testing"

	"github.com/samber/mo"
	"go.mongodb.org/mongo-driver/v2/bson"

	taskdomain "todoe/domain/task/domain"
	"todoe/internal/audit/domain"
	"todoe/internal/event"
)

// ── fakes ────────────────────────────────────────────────────────────

type fakeAuditRepo struct {
	saveCalls []domain.AuditRecord
	saveRes   mo.Result[struct{}]
}

func (f *fakeAuditRepo) Save(ctx context.Context, r domain.AuditRecord) mo.Result[struct{}] {
	f.saveCalls = append(f.saveCalls, r)
	return f.saveRes
}

// ── tests ────────────────────────────────────────────────────────────

func TestHandler_Handle_SavesAuditRecordForTaskEvent(t *testing.T) {
	repo := &fakeAuditRepo{saveRes: mo.Ok(struct{}{})}
	h := NewHandler(repo)

	task := taskdomain.Task{
		ID:     bson.NewObjectID(),
		Title:  "test task",
		Status: taskdomain.StatusDone,
	}

	err := h.Handle(context.Background(), event.Event{
		Type:    taskdomain.EventCreated,
		Payload: task,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.saveCalls) != 1 {
		t.Fatalf("expected 1 Save call, got %d", len(repo.saveCalls))
	}
	rec := repo.saveCalls[0]
	if rec.TaskID != task.ID {
		t.Errorf("TaskID=%v, want %v", rec.TaskID, task.ID)
	}
	if rec.EventType != taskdomain.EventCreated {
		t.Errorf("EventType=%q, want %q", rec.EventType, taskdomain.EventCreated)
	}
	if rec.Status != string(task.Status) {
		t.Errorf("Status=%q, want %q", rec.Status, task.Status)
	}
	if rec.Time.IsZero() {
		t.Error("expected non-zero Time")
	}
}

func TestHandler_Handle_IgnoresNonTaskPayload(t *testing.T) {
	repo := &fakeAuditRepo{saveRes: mo.Ok(struct{}{})}
	h := NewHandler(repo)

	err := h.Handle(context.Background(), event.Event{
		Type:    "something.else",
		Payload: "not a task",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.saveCalls) != 0 {
		t.Fatalf("expected no Save call, got %d", len(repo.saveCalls))
	}
}

func TestHandler_Handle_ReturnsErrorWhenSaveFails(t *testing.T) {
	repo := &fakeAuditRepo{saveRes: mo.Err[struct{}](errors.New("db down"))}
	h := NewHandler(repo)

	err := h.Handle(context.Background(), event.Event{
		Type:    taskdomain.EventCreated,
		Payload: taskdomain.Task{ID: bson.NewObjectID(), Status: taskdomain.StatusPending},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
