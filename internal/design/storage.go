package design

import "context"

type StateStore interface {
	Get(ctx context.Context, id string) (Status, error)
	Save(ctx context.Context, id string, status Status) error
}

type ReportRepository interface {
	Create(ctx context.Context, report Report) (Report, error)
	ListByProjectID(ctx context.Context, projectID string) ([]Report, error)
}
