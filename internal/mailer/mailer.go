package mailer

import "context"

type SendResult struct {
	Provider string
	Accepted bool
	ID       string
}

type Mailer interface {
	Send(ctx context.Context, message Message) (SendResult, error)
}
