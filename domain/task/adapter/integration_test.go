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

	"todoe/domain/task/domain"
	"todoe/internal/event"
)

// setupRepo connects to mongo (MONGO_URI env or local default). Skips the test
// when mongo isn't reachable, so `go test ./...` stays green on a dev box
// without compose running but exercises the full path when it is.
// setupRepo connects to mongo (MONGO_URI env or local default). Skips the test
// when mongo isn't reachable, so `go test ./...` stays green on a dev box
// without compose running but exercises the full path when it is.
func setupRepo(t *testing.T) *MongoRepository {
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

// removeAfterTest deletes the task with id from the tasks collection at end of test,
// keeping the shared `todoe.tasks` collection clean.
func removeAfterTest(t *testing.T, repo *MongoRepository, id bson.ObjectID) {
	t.Helper()
	t.Cleanup(func() {
		col, err := repo.collection()
		if err != nil {
			return
		}
		_, _ = col.DeleteOne(context.Background(), bson.D{{Key: "_id", Value: id}})
	})
}

func sampleTask() domain.Task {
	return domain.Task{
		ID:        bson.NewObjectID(),
		Title:     "integration",
		Status:    domain.StatusPending,
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}
}

func TestMongoRepository_SaveAndFindByID(t *testing.T) {
	repo := setupRepo(t)
	task := sampleTask()
	removeAfterTest(t, repo, task.ID)

	ctx := context.Background()
	if r := repo.Save(ctx, task); r.IsError() {
		t.Fatalf("Save: %v", r.Error())
	}

	res := repo.FindByID(ctx, task.ID)
	if res.IsError() {
		t.Fatalf("FindByID: %v", res.Error())
	}
	got := res.MustGet()
	if got.ID != task.ID || got.Title != task.Title || got.Status != task.Status {
		t.Errorf("got %+v, want %+v", got, task)
	}
}

func TestMongoRepository_FindAll_IncludesSavedTask(t *testing.T) {
	repo := setupRepo(t)
	task := sampleTask()
	removeAfterTest(t, repo, task.ID)

	ctx := context.Background()
	if r := repo.Save(ctx, task); r.IsError() {
		t.Fatalf("Save: %v", r.Error())
	}

	res := repo.FindAll(ctx)
	if res.IsError() {
		t.Fatalf("FindAll: %v", res.Error())
	}
	found := false
	for _, x := range res.MustGet() {
		if x.ID == task.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("saved task not found in FindAll")
	}
}

func TestMongoRepository_ChangeStatusViaAppend(t *testing.T) {
	repo := setupRepo(t)
	task := sampleTask()
	removeAfterTest(t, repo, task.ID)

	ctx := context.Background()
	if r := repo.Save(ctx, task); r.IsError() {
		t.Fatalf("Save: %v", r.Error())
	}

	// "Update" = save a new record referencing the original ID
	next := task.ChangeStatus(domain.StatusInProgress)
	removeAfterTest(t, repo, next.ID)
	if r := repo.Save(ctx, next); r.IsError() {
		t.Fatalf("Save status change: %v", r.Error())
	}

	// The latest record has the new status
	res := repo.FindByID(ctx, next.ID)
	if res.IsError() {
		t.Fatalf("FindByID: %v", res.Error())
	}
	if res.MustGet().Status != domain.StatusInProgress {
		t.Errorf("status=%v, want in_progress", res.MustGet().Status)
	}
}

func TestSaveHandler_PersistsTaskFromBus(t *testing.T) {
	repo := setupRepo(t)
	bus := event.NewEventBus()
	bus.Subscribe(domain.EventCreated, NewSaveHandler(repo))

	task := sampleTask()
	removeAfterTest(t, repo, task.ID)

	bus.Publish(context.Background(), event.Event{
		Type:    domain.EventCreated,
		Payload: task,
	})

	res := repo.FindByID(context.Background(), task.ID)
	if res.IsError() {
		t.Fatalf("FindByID: %v", res.Error())
	}
	if res.MustGet().Title != task.Title {
		t.Errorf("title=%q, want %q", res.MustGet().Title, task.Title)
	}
}

func TestSaveHandler_PersistsStatusChangeFromBus(t *testing.T) {
	repo := setupRepo(t)
	bus := event.NewEventBus()
	bus.Subscribe(domain.EventStatusChanged, NewSaveHandler(repo))

	task := sampleTask()
	removeAfterTest(t, repo, task.ID)
	if r := repo.Save(context.Background(), task); r.IsError() {
		t.Fatalf("Save: %v", r.Error())
	}

	next := task.ChangeStatus(domain.StatusDone)
	removeAfterTest(t, repo, next.ID)

	bus.Publish(context.Background(), event.Event{
		Type:    domain.EventStatusChanged,
		Payload: next,
	})

	res := repo.FindByID(context.Background(), next.ID)
	if res.IsError() {
		t.Fatalf("FindByID: %v", res.Error())
	}
	if res.MustGet().Status != domain.StatusDone {
		t.Errorf("status=%v, want done", res.MustGet().Status)
	}
}
