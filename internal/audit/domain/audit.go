package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type AuditRecord struct {
	ID        bson.ObjectID `bson:"_id"`
	TaskID    bson.ObjectID `bson:"taskID"`
	EventType string        `bson:"eventType"`
	Status    string        `bson:"status,omitempty"`
	Time      time.Time     `bson:"time"`
}
