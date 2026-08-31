package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gamidoc/backend/internal/design"
	goredis "github.com/redis/go-redis/v9"
)

type DesignRepository struct {
	client *Client
	ttl    time.Duration
}

func NewDesignRepository(client *Client, ttl time.Duration) *DesignRepository {
	return &DesignRepository{
		client: client,
		ttl:    ttl,
	}
}

func (r *DesignRepository) Get(ctx context.Context, id string) (design.Status, error) {
	value, err := r.client.Raw().Get(ctx, r.key(id)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return design.NewInitialStatus(), nil
		}
		return design.Status{}, err
	}

	var status design.Status
	if err := json.Unmarshal([]byte(value), &status); err != nil {
		return design.Status{}, err
	}
	if status.Sections == nil {
		status.Sections = map[string]design.SectionState{}
	}

	return status, nil
}

func (r *DesignRepository) Save(ctx context.Context, id string, status design.Status) error {
	payload, err := json.Marshal(status)
	if err != nil {
		return err
	}

	return saveKeepTTL.Run(ctx, r.client.Raw(), []string{r.key(id)}, payload, r.ttl.Milliseconds()).Err()
}

var saveKeepTTL = goredis.NewScript(`
redis.call('SET', KEYS[1], ARGV[1], 'KEEPTTL')
if redis.call('PTTL', KEYS[1]) < 0 then
	redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
return 1
`)

func (r *DesignRepository) key(id string) string {
	return "design:" + id
}
