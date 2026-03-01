package settings

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/ghobs91/lodestone/internal/database/dao"
	"github.com/ghobs91/lodestone/internal/lazy"
	"gorm.io/gorm"
)

const tmdbApiKeySettingsKey = "tmdb_api_key"

// TmdbApiKeySettings holds the user-configured TMDB API key.
type TmdbApiKeySettings struct {
	ApiKey string `json:"apiKey"`
}

// TmdbApiKeySettingsStore reads and writes TMDB API key settings.
type TmdbApiKeySettingsStore interface {
	Get(ctx context.Context) (TmdbApiKeySettings, error)
	Save(ctx context.Context, s TmdbApiKeySettings) (TmdbApiKeySettings, error)
}

type tmdbApiKeySettingsStore struct {
	dao *dao.Query
}

func NewTmdbApiKeySettingsStore(d lazy.Lazy[*dao.Query]) lazy.Lazy[TmdbApiKeySettingsStore] {
	return lazy.New(func() (TmdbApiKeySettingsStore, error) {
		q, err := d.Get()
		if err != nil {
			return nil, err
		}
		return &tmdbApiKeySettingsStore{dao: q}, nil
	})
}

func (s *tmdbApiKeySettingsStore) Get(ctx context.Context) (TmdbApiKeySettings, error) {
	kv, err := s.dao.KeyValue.WithContext(ctx).
		Where(s.dao.KeyValue.Key.Eq(tmdbApiKeySettingsKey)).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return TmdbApiKeySettings{}, nil
		}
		return TmdbApiKeySettings{}, err
	}
	var ts TmdbApiKeySettings
	if err := json.Unmarshal([]byte(kv.Value), &ts); err != nil {
		return TmdbApiKeySettings{}, err
	}
	return ts, nil
}

func (s *tmdbApiKeySettingsStore) Save(ctx context.Context, ts TmdbApiKeySettings) (TmdbApiKeySettings, error) {
	data, err := json.Marshal(ts)
	if err != nil {
		return TmdbApiKeySettings{}, err
	}
	now := time.Now()
	db := s.dao.KeyValue.UnderlyingDB()
	err = db.WithContext(ctx).Exec(
		`INSERT INTO key_values (key, value, created_at, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at`,
		tmdbApiKeySettingsKey, string(data), now, now,
	).Error
	if err != nil {
		return TmdbApiKeySettings{}, err
	}
	return ts, nil
}
