package mailer

import "context"

type NoopMailer struct{}

func NewNoopMailer() *NoopMailer {
	return &NoopMailer{}
}

func (m *NoopMailer) Send(ctx context.Context, message Message) (SendResult, error) {
	return SendResult{
		Provider: "noop",
		Accepted: false,
		ID:       "",
	}, nil
}
