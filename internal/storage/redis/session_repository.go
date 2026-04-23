package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gamidoc/backend/internal/session"
	"github.com/gamidoc/backend/internal/wizard"
	goredis "github.com/redis/go-redis/v9"
)

type SessionRepository struct {
	client *Client
	ttl    time.Duration
}

func NewSessionRepository(client *Client, ttl time.Duration) *SessionRepository {
	return &SessionRepository{
		client: client,
		ttl:    ttl,
	}
}

func (r *SessionRepository) Create(ctx context.Context, input session.Session) (session.Session, error) {
	payload, err := json.Marshal(input)
	if err != nil {
		return session.Session{}, err
	}

	key := r.key(input.ID)
	if err := r.client.Raw().Set(ctx, key, payload, r.ttl).Err(); err != nil {
		return session.Session{}, err
	}

	return input, nil
}

func (r *SessionRepository) FindByID(ctx context.Context, id string) (session.Session, error) {
	key := r.key(id)

	value, err := r.client.Raw().Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return session.Session{}, session.ErrSessionNotFound
		}
		return session.Session{}, err
	}

	var found session.Session
	if err := json.Unmarshal([]byte(value), &found); err != nil {
		return session.Session{}, err
	}

	return found, nil
}

func (r *SessionRepository) FindWizardByID(ctx context.Context, id string) (wizard.Status, error) {
	found, err := r.FindByID(ctx, id)
	if err != nil {
		return wizard.Status{}, err
	}
	return found.Wizard, nil
}

func (r *SessionRepository) UpdateWizard(ctx context.Context, id string, status wizard.Status) (session.Session, error) {
	found, err := r.FindByID(ctx, id)
	if err != nil {
		return session.Session{}, err
	}

	found.Wizard = status
	found.PDFURL = nil

	payload, err := json.Marshal(found)
	if err != nil {
		return session.Session{}, err
	}

	key := r.key(id)
	if err := r.client.Raw().SetArgs(ctx, key, payload, goredis.SetArgs{KeepTTL: true}).Err(); err != nil {
		return session.Session{}, err
	}

	return found, nil
}

func (r *SessionRepository) UpdatePDFURL(ctx context.Context, id string, pdfURL string) (session.Session, error) {
	found, err := r.FindByID(ctx, id)
	if err != nil {
		return session.Session{}, err
	}

	found.PDFURL = &pdfURL

	payload, err := json.Marshal(found)
	if err != nil {
		return session.Session{}, err
	}

	key := r.key(id)
	if err := r.client.Raw().SetArgs(ctx, key, payload, goredis.SetArgs{KeepTTL: true}).Err(); err != nil {
		return session.Session{}, err
	}

	return found, nil
}

func (r *SessionRepository) key(id string) string {
	return "session:" + id
}
