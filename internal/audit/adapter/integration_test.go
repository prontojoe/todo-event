package adapter

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/samber/mo"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	taskdomain "todoe/domain/task/domain"
	"todoe/internal/audit/domain"
	"todoe/internal/event"
)

// setupAuditRepo connects to mongo (MONGO_URI env or local default).
func setupAuditRepo(t *testing.T) *MongoRepository {
	t.Helper()

	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://root:root@localhost:27017"
	}

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Skipf("mongo unavailable; skipping integration test: %v", err)
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		t.Skipf("mongo unavailable; skipping integration test: %v", err)
	}
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })

	clientIO := mo.NewIOEither(func() (*mongo.Client, error) { return client, nil })
	return NewMongoRepository(clientIO)
}

func TestMongoRepository_SaveAndFindByTaskID(t *testing.T) {
	repo := setupAuditRepo(t)

	taskID := bson.NewObjectID()
	record := domain.AuditRecord{
		TaskID:    taskID,
		EventType: taskdomain.EventCreated,
		Status:    string(taskdomain.StatusPending),
		Time:      time.Now().UTC(),
	}

	ctx := context.Background()
	if r := repo.Save(ctx, record); r.IsError() {
		t.Fatalf("Save: %v", r.Error())
	}

	res := repo.FindByTaskID(ctx, taskID)
	if res.IsError() {
		t.Fatalf("FindByTaskID: %v", res.Error())
	}
	records := res.MustGet()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	got := records[0]
	if got.TaskID != taskID {
		t.Errorf("TaskID=%v, want %v", got.TaskID, taskID)
	}
	if got.EventType != taskdomain.EventCreated {
		t.Errorf("EventType=%q, want %q", got.EventType, taskdomain.EventCreated)
	}
	if got.Status != string(taskdomain.StatusPending) {
		t.Errorf("Status=%q, want %q", got.Status, taskdomain.StatusPending)
	}
}

func TestMongoRepository_FindByTaskID_ReturnsMultipleRecords(t *testing.T) {
	repo := setupAuditRepo(t)
	taskID := bson.NewObjectID()
	ctx := context.Background()

	records := []domain.AuditRecord{
		{TaskID: taskID, EventType: taskdomain.EventCreated, Status: string(taskdomain.StatusPending), Time: time.Now().UTC()},
		{TaskID: taskID, EventType: taskdomain.EventStatusChanged, Status: string(taskdomain.StatusDone), Time: time.Now().UTC()},
	}

	for _, rec := range records {
		if r := repo.Save(ctx, rec); r.IsError() {
			t.Fatalf("Save: %v", r.Error())
		}
	}

	res := repo.FindByTaskID(ctx, taskID)
	if res.IsError() {
		t.Fatalf("FindByTaskID: %v", res.Error())
	}
	got := res.MustGet()
	if len(got) != 2 {
		t.Fatalf("expected 2 records, got %d", len(got))
	}
}

func TestMongoRepository_FindByTaskID_ReturnsEmptyForUnknownID(t *testing.T) {
	repo := setupAuditRepo(t)

	res := repo.FindByTaskID(context.Background(), bson.NewObjectID())
	if res.IsError() {
		t.Fatalf("FindByTaskID: %v", res.Error())
	}
	if len(res.MustGet()) != 0 {
		t.Errorf("expected 0 records, got %d", len(res.MustGet()))
	}
}

func TestAuditHandler_PersistsRecordFromBus(t *testing.T) {
	repo := setupAuditRepo(t)
	bus := event.NewEventBus()
	h := NewHandler(repo)
	bus.Subscribe(taskdomain.EventCreated, h.Handle)

	task := taskdomain.Task{
		ID:     bson.NewObjectID(),
		Title:  "bus test",
		Status: taskdomain.StatusPending,
	}

	bus.Publish(context.Background(), event.Event{
		Type:    taskdomain.EventCreated,
		Payload: task,
	})

	res := repo.FindByTaskID(context.Background(), task.ID)
	if res.IsError() {
		t.Fatalf("FindByTaskID: %v", res.Error())
	}
	records := res.MustGet()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].EventType != taskdomain.EventCreated {
		t.Errorf("EventType=%q, want %q", records[0].EventType, taskdomain.EventCreated)
	}
	if records[0].Status != string(task.Status) {
		t.Errorf("Status=%q, want %q", records[0].Status, task.Status)
	}
}
