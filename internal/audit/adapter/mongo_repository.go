package adapter

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/samber/mo"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"todoe/internal/audit/domain"
)

type MongoRepository struct {
	clientIO    mo.IOEither[*mongo.Client]
	once        sync.Once
	cached      mo.Either[error, *mongo.Client]
	initialized atomic.Bool
}

func NewMongoRepository(clientIO mo.IOEither[*mongo.Client]) *MongoRepository {
	return &MongoRepository{clientIO: clientIO}
}

func (r *MongoRepository) getClient() mo.Either[error, *mongo.Client] {
	r.once.Do(func() {
		r.cached = r.clientIO.Run()
		r.initialized.Store(true)
	})
	return r.cached
}

func (r *MongoRepository) collection() (*mongo.Collection, error) {
	either := r.getClient()
	if either.IsLeft() {
		return nil, either.MustLeft()
	}
	return either.MustRight().Database("todoe").Collection("audits"), nil
}

func (r *MongoRepository) Save(ctx context.Context, record domain.AuditRecord) mo.Result[struct{}] {
	col, err := r.collection()
	if err != nil {
		return mo.Err[struct{}](err)
	}
	record.ID = bson.NewObjectID()
	if _, err := col.InsertOne(ctx, record); err != nil {
		return mo.Err[struct{}](err)
	}
	return mo.Ok(struct{}{})
}

func (r *MongoRepository) FindByTaskID(ctx context.Context, taskID bson.ObjectID) mo.Result[[]domain.AuditRecord] {
	col, err := r.collection()
	if err != nil {
		return mo.Err[[]domain.AuditRecord](err)
	}
	cursor, err := col.Find(ctx, bson.D{{Key: "taskID", Value: taskID}})
	if err != nil {
		return mo.Err[[]domain.AuditRecord](err)
	}
	defer cursor.Close(ctx)

	var records []domain.AuditRecord
	if err := cursor.All(ctx, &records); err != nil {
		return mo.Err[[]domain.AuditRecord](err)
	}
	return mo.Ok(records)
}
