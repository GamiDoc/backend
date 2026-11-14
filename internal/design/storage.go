package design

import "context"

type StateStore interface {
	Get(ctx context.Context, id string) (Status, error)
	Save(ctx context.Context, id string, status Status) error
}

type ReportRepository interface {
	Create(ctx context.Context, report Report) (Report, error)
	ListByProjectID(ctx context.Context, projectID string) ([]Report, error)
	// Delete removes a report of the project and returns it (ErrReportNotFound if none).
	Delete(ctx context.Context, projectID string, reportID string) (Report, error)
	// UpdateURL points a report to its file after a rename (ErrReportNotFound if none).
	UpdateURL(ctx context.Context, projectID string, reportID string, url string) (Report, error)
}
