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

const classifierSettingsKey = "classifier_settings"

// ClassifierSettings holds the user-configurable classifier preferences.
type ClassifierSettings struct {
	DeleteXxx      bool     `json:"deleteXxx"`
	BannedKeywords []string `json:"bannedKeywords"`
}

// ClassifierSettingsStore reads and writes classifier settings from the DB.
type ClassifierSettingsStore interface {
	Get(ctx context.Context) (ClassifierSettings, error)
	Save(ctx context.Context, s ClassifierSettings) (ClassifierSettings, error)
}

type classifierSettingsStore struct {
	dao *dao.Query
}

func NewClassifierSettingsStore(d lazy.Lazy[*dao.Query]) lazy.Lazy[ClassifierSettingsStore] {
	return lazy.New(func() (ClassifierSettingsStore, error) {
		q, err := d.Get()
		if err != nil {
			return nil, err
		}
		return &classifierSettingsStore{dao: q}, nil
	})
}

func (s *classifierSettingsStore) Get(ctx context.Context) (ClassifierSettings, error) {
	kv, err := s.dao.KeyValue.WithContext(ctx).
		Where(s.dao.KeyValue.Key.Eq(classifierSettingsKey)).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ClassifierSettings{BannedKeywords: []string{}}, nil
		}
		return ClassifierSettings{}, err
	}
	var cs ClassifierSettings
	if err := json.Unmarshal([]byte(kv.Value), &cs); err != nil {
		return ClassifierSettings{}, err
	}
	if cs.BannedKeywords == nil {
		cs.BannedKeywords = []string{}
	}
	return cs, nil
}

func (s *classifierSettingsStore) Save(ctx context.Context, cs ClassifierSettings) (ClassifierSettings, error) {
	if cs.BannedKeywords == nil {
		cs.BannedKeywords = []string{}
	}
	data, err := json.Marshal(cs)
	if err != nil {
		return ClassifierSettings{}, err
	}
	now := time.Now()
	db := s.dao.KeyValue.UnderlyingDB()
	err = db.WithContext(ctx).Exec(
		`INSERT INTO key_values (key, value, created_at, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at`,
		classifierSettingsKey, string(data), now, now,
	).Error
	if err != nil {
		return ClassifierSettings{}, err
	}
	return cs, nil
}

