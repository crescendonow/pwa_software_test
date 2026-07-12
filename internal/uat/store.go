package uat

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")

type Store interface {
	Health(ctx context.Context) error
	ListReferences(ctx context.Context) (ReferenceData, error)
	ListTestCases(ctx context.Context, testSuite string) ([]TestCase, error)
	ListSessions(ctx context.Context, filters SessionFilters) ([]Session, error)
	DeleteSession(ctx context.Context, sessionID int64, uid string) error
	CreateSession(ctx context.Context, input CreateSessionInput) (Session, error)
	GetSessionResults(ctx context.Context, sessionID int64) (SessionResults, error)
	UpdateResult(ctx context.Context, resultID int64, input UpdateResultInput) (Result, error)
	Report(ctx context.Context, filters ReportFilters) ([]ReportRow, error)
	Close()
}
