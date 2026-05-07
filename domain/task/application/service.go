package application

import (
	"context"
	"errors"

	"github.com/samber/mo"
	"go.mongodb.org/mongo-driver/v2/bson"

	"todoe/domain/task/domain"
	"todoe/domain/task/port"
	"todoe/internal/event"
)

var (
	ErrInvalidTitle  = errors.New("title must not be empty")
	ErrInvalidStatus = errors.New("status must be one of: pending, in_progress, done")
)

type Service struct {
	repo      port.Repository
	publisher event.Publisher
}

var _ port.UseCase = (*Service)(nil)

func NewService(repo port.Repository, publisher event.Publisher) *Service {
	return &Service{repo: repo, publisher: publisher}
}

func validateTitle(title string) error {
	if title == "" {
		return ErrInvalidTitle
	}
	return nil
}

func validateStatus(s domain.Status) error {
	switch s {
	case domain.StatusPending, domain.StatusInProgress, domain.StatusDone:
		return nil
	}
	return ErrInvalidStatus
}

func (s *Service) CreateTask(ctx context.Context, title string) mo.Result[domain.Task] {
	if err := validateTitle(title); err != nil {
		return mo.Err[domain.Task](err)
	}
	task := domain.NewTask(title)
	s.publisher.Publish(ctx, event.Event{
		Type:    domain.EventCreated,
		Payload: task,
	})
	return mo.Ok(task)
}

func (s *Service) ListTasks(ctx context.Context) mo.Result[[]domain.Task] {
	return s.repo.FindAll(ctx)
}

func (s *Service) GetTask(ctx context.Context, id bson.ObjectID) mo.Result[domain.Task] {
	return s.repo.FindByID(ctx, id)
}

func (s *Service) ChangeStatus(ctx context.Context, task domain.Task, status domain.Status) mo.Result[domain.Task] {
	if err := validateStatus(status); err != nil {
		return mo.Err[domain.Task](err)
	}
	next := task.ChangeStatus(status)
	s.publisher.Publish(ctx, event.Event{
		Type:    domain.EventStatusChanged,
		Payload: next,
	})
	return mo.Ok(next)
}
