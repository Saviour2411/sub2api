package repository

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/setting"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type settingRepository struct {
	client *ent.Client
}

// SetMultipleWithMonotonicRevision 在同一事务内推进策略代次并保存配置。
func (r *settingRepository) SetMultipleWithMonotonicRevision(
	ctx context.Context,
	settingsMap map[string]string,
	revisionKey string,
	fingerprintKey string,
	initialRevision int64,
	currentFingerprintFallback string,
	desiredFingerprint string,
) (int64, error) {
	if r == nil || r.client == nil {
		return 0, fmt.Errorf("设置仓储未初始化")
	}
	if strings.TrimSpace(revisionKey) == "" || strings.TrimSpace(fingerprintKey) == "" ||
		strings.TrimSpace(desiredFingerprint) == "" || initialRevision <= 0 {
		return 0, fmt.Errorf("策略代次参数无效")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	now := time.Now()
	if err := client.Setting.Create().
		SetKey(revisionKey).
		SetValue(strconv.FormatInt(initialRevision, 10)).
		SetUpdatedAt(now).
		OnConflictColumns(setting.FieldKey).
		Ignore().
		Exec(ctx); err != nil {
		return 0, err
	}
	stored, err := client.Setting.Query().Where(setting.KeyEQ(revisionKey)).ForUpdate().Only(ctx)
	if err != nil {
		return 0, err
	}
	current, err := strconv.ParseInt(strings.TrimSpace(stored.Value), 10, 64)
	if err != nil || current < initialRevision {
		return 0, fmt.Errorf("持久化策略代次无效")
	}
	currentFingerprint := strings.TrimSpace(currentFingerprintFallback)
	storedFingerprint, fingerprintErr := client.Setting.Query().Where(setting.KeyEQ(fingerprintKey)).Only(ctx)
	if fingerprintErr == nil {
		currentFingerprint = strings.TrimSpace(storedFingerprint.Value)
	} else if !ent.IsNotFound(fingerprintErr) {
		return 0, fingerprintErr
	}
	next := current
	if currentFingerprint != strings.TrimSpace(desiredFingerprint) {
		if current == math.MaxInt64 {
			return 0, fmt.Errorf("持久化策略代次已耗尽")
		}
		next++
	}
	values := make(map[string]string, len(settingsMap)+2)
	for key, value := range settingsMap {
		values[key] = value
	}
	values[revisionKey] = strconv.FormatInt(next, 10)
	values[fingerprintKey] = strings.TrimSpace(desiredFingerprint)
	builders := make([]*ent.SettingCreate, 0, len(values))
	for key, value := range values {
		builders = append(builders, client.Setting.Create().SetKey(key).SetValue(value).SetUpdatedAt(now))
	}
	if err := client.Setting.CreateBulk(builders...).
		OnConflictColumns(setting.FieldKey).
		UpdateNewValues().
		Exec(ctx); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return next, nil
}

func NewSettingRepository(client *ent.Client) service.SettingRepository {
	return &settingRepository{client: client}
}

func (r *settingRepository) Get(ctx context.Context, key string) (*service.Setting, error) {
	m, err := r.client.Setting.Query().Where(setting.KeyEQ(key)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, service.ErrSettingNotFound
		}
		return nil, err
	}
	return &service.Setting{
		ID:        m.ID,
		Key:       m.Key,
		Value:     m.Value,
		UpdatedAt: m.UpdatedAt,
	}, nil
}

func (r *settingRepository) GetValue(ctx context.Context, key string) (string, error) {
	setting, err := r.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

func (r *settingRepository) Set(ctx context.Context, key, value string) error {
	now := time.Now()
	return r.client.Setting.
		Create().
		SetKey(key).
		SetValue(value).
		SetUpdatedAt(now).
		OnConflictColumns(setting.FieldKey).
		UpdateNewValues().
		Exec(ctx)
}

func (r *settingRepository) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return map[string]string{}, nil
	}
	settings, err := r.client.Setting.Query().Where(setting.KeyIn(keys...)).All(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

func (r *settingRepository) SetMultiple(ctx context.Context, settings map[string]string) error {
	if len(settings) == 0 {
		return nil
	}

	now := time.Now()
	builders := make([]*ent.SettingCreate, 0, len(settings))
	for key, value := range settings {
		builders = append(builders, r.client.Setting.Create().SetKey(key).SetValue(value).SetUpdatedAt(now))
	}
	return r.client.Setting.
		CreateBulk(builders...).
		OnConflictColumns(setting.FieldKey).
		UpdateNewValues().
		Exec(ctx)
}

func (r *settingRepository) GetAll(ctx context.Context) (map[string]string, error) {
	settings, err := r.client.Setting.Query().All(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

func (r *settingRepository) Delete(ctx context.Context, key string) error {
	_, err := r.client.Setting.Delete().Where(setting.KeyEQ(key)).Exec(ctx)
	return err
}
