package domain

import "time"

type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobRetry     JobStatus = "retry"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
)

type OutboxJob struct {
	ID          string
	Kind        string
	AggregateID string
	Payload     []byte
	Status      JobStatus
	Attempts    int
	MaxAttempts int
	AvailableAt time.Time
	LockedAt    *time.Time
	LastError   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type AuditEvent struct {
	ID         string
	ActorID    string
	RequestID  string
	Action     string
	ObjectType string
	ObjectID   string
	Result     string
	Detail     string
	CreatedAt  time.Time
}

type IdempotencyRecord struct {
	Scope       string
	Key         string
	PayloadHash string
	StatusCode  int
	Response    []byte
	CreatedAt   time.Time
	ExpiresAt   time.Time
}
