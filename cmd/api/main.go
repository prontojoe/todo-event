package main

import (
	"context"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/samber/mo"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	healthadapter "todoe/internal/health/adapter"
	healthhttp "todoe/internal/health/adapter/http"
	healthapp "todoe/internal/health/application"

	"todoe/internal/event"

	auditadapter "todoe/internal/audit/adapter"

	taskadapter "todoe/domain/task/adapter"
	taskdomain "todoe/domain/task/domain"
	taskhttp "todoe/domain/task/adapter/http"
	taskapplication "todoe/domain/task/application"
)

func main() {
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://root:root@localhost:27017"
	}

	clientIO := mo.NewIOEither(func() (*mongo.Client, error) {
		return mongo.Connect(options.Client().ApplyURI(mongoURI))
	})

	healthRepo := healthadapter.NewMongoRepository(clientIO)
	defer healthRepo.Disconnect(context.Background())

	healthService := healthapp.NewService(healthRepo)
	healthHandler := healthhttp.NewHandler(healthService)

	taskRepo := taskadapter.NewMongoRepository(clientIO)
	taskBus := event.NewEventBus()
	taskBus.Subscribe(taskdomain.EventCreated, taskadapter.NewSaveHandler(taskRepo))
	taskBus.Subscribe(taskdomain.EventStatusChanged, taskadapter.NewSaveHandler(taskRepo))
	auditRepo := auditadapter.NewMongoRepository(clientIO)
	taskBus.Subscribe(taskdomain.EventCreated, auditadapter.NewHandler(auditRepo).Handle)
	taskBus.Subscribe(taskdomain.EventStatusChanged, auditadapter.NewHandler(auditRepo).Handle)
	taskService := taskapplication.NewService(taskRepo, taskBus)
	taskHandler := taskhttp.NewHandler(taskService)

	app := fiber.New()
	app.Get("/health", healthHandler.CheckHealth)
	app.Post("/tasks", taskHandler.Create)
	app.Get("/tasks", taskHandler.List)
	app.Get("/tasks/:id", taskHandler.Detail)
	app.Patch("/tasks/:id/status", taskHandler.ChangeStatus)

	log.Fatal(app.Listen(":3000"))
}
