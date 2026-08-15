//go:build unit

package service

import (
	"context"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type canvasAPIKeyRepoStub struct {
	authRepoStub
	mu     sync.Mutex
	keys   []APIKey
	nextID int64
}

func (s *canvasAPIKeyRepoStub) Create(_ context.Context, key *APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key.Purpose == APIKeyPurposeInfiniteCanvas && key.GroupID != nil {
		for i := range s.keys {
			stored := &s.keys[i]
			if stored.UserID == key.UserID && stored.GroupID != nil && *stored.GroupID == *key.GroupID && stored.Purpose == APIKeyPurposeInfiniteCanvas {
				return ErrAPIKeyExists
			}
		}
	}
	s.nextID++
	key.ID = s.nextID
	s.keys = append(s.keys, *key)
	return nil
}

func (s *canvasAPIKeyRepoStub) ExistsByKey(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.keys {
		if s.keys[i].Key == key {
			return true, nil
		}
	}
	return false, nil
}

func (s *canvasAPIKeyRepoStub) CountByUserID(_ context.Context, userID int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int64
	for i := range s.keys {
		if s.keys[i].UserID == userID {
			count++
		}
	}
	return count, nil
}

func (s *canvasAPIKeyRepoStub) ListAllByUserID(_ context.Context, userID int64, filters APIKeyListFilters) ([]APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]APIKey, 0, len(s.keys))
	for i := range s.keys {
		key := s.keys[i]
		if key.UserID != userID {
			continue
		}
		if filters.GroupID != nil && (key.GroupID == nil || *key.GroupID != *filters.GroupID) {
			continue
		}
		result = append(result, key)
	}
	return result, nil
}

type canvasGroupRepoStub struct {
	stubGroupRepoForAvailable
}

func (s *canvasGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	for i := range s.activeGroups {
		if s.activeGroups[i].ID == id {
			return &s.activeGroups[i], nil
		}
	}
	return nil, ErrGroupNotFound
}

type canvasUserSubRepoStub struct {
	userSubRepoNoop
}

func (*canvasUserSubRepoStub) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return nil, nil
}

func newCanvasAPIKeyService(repo *canvasAPIKeyRepoStub, users map[int64]*User, groups []Group) *APIKeyService {
	return NewAPIKeyService(
		repo,
		&userRepoStub{usersByID: users},
		&canvasGroupRepoStub{stubGroupRepoForAvailable: stubGroupRepoForAvailable{activeGroups: groups}},
		&canvasUserSubRepoStub{},
		nil,
		nil,
		&config.Config{},
	)
}

func publicCanvasGroups() []Group {
	return []Group{
		{ID: 1, Name: "OpenAI", Platform: PlatformOpenAI, Status: StatusActive},
		{ID: 2, Name: "Gemini", Platform: PlatformGemini, Status: StatusActive},
		{ID: 3, Name: "Anthropic", Platform: PlatformAnthropic, Status: StatusActive},
		{ID: 4, Name: "Grok 专属", Platform: PlatformGrok, Status: StatusActive, IsExclusive: true},
	}
}

func TestGetCanvasBootstrapFiltersPlatformsAndPermissions(t *testing.T) {
	svc := newCanvasAPIKeyService(&canvasAPIKeyRepoStub{}, map[int64]*User{1: {ID: 1}}, publicCanvasGroups())

	bootstrap, err := svc.GetCanvasBootstrap(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, bootstrap.Groups, 2)
	assert.Equal(t, int64(1), bootstrap.UserID)
	assert.Equal(t, []int64{1, 2}, []int64{bootstrap.Groups[0].ID, bootstrap.Groups[1].ID})
	assert.Equal(t, []string{"openai", "gemini"}, []string{bootstrap.Groups[0].APIFormat, bootstrap.Groups[1].APIFormat})
}

func TestResolveCanvasCredentialReusesGeneralKey(t *testing.T) {
	groupID := int64(1)
	repo := &canvasAPIKeyRepoStub{keys: []APIKey{{ID: 7, UserID: 1, GroupID: &groupID, Key: "sk-general", Status: StatusActive, Purpose: APIKeyPurposeGeneral}}}
	svc := newCanvasAPIKeyService(repo, map[int64]*User{1: {ID: 1}}, publicCanvasGroups())

	credential, err := svc.ResolveCanvasCredential(context.Background(), 1, groupID, "192.0.2.10")
	require.NoError(t, err)
	assert.Equal(t, "sk-general", credential.APIKey)
	assert.Len(t, repo.keys, 1)
}

func TestResolveCanvasCredentialCreatesManagedKey(t *testing.T) {
	repo := &canvasAPIKeyRepoStub{}
	svc := newCanvasAPIKeyService(repo, map[int64]*User{1: {ID: 1}}, publicCanvasGroups())

	credential, err := svc.ResolveCanvasCredential(context.Background(), 1, 1, "192.0.2.10")
	require.NoError(t, err)
	require.Len(t, repo.keys, 1)
	created := repo.keys[0]
	assert.Equal(t, credential.APIKey, created.Key)
	assert.Equal(t, APIKeyPurposeInfiniteCanvas, created.Purpose)
	assert.Equal(t, "无限画布自动创建", created.Name)
	assert.Zero(t, created.Quota)
	assert.Nil(t, created.ExpiresAt)
	assert.Empty(t, created.IPWhitelist)
	assert.Zero(t, created.RateLimit5h)
}

func TestResolveCanvasCredentialConcurrentCreationIsUnique(t *testing.T) {
	repo := &canvasAPIKeyRepoStub{}
	svc := newCanvasAPIKeyService(repo, map[int64]*User{1: {ID: 1}}, publicCanvasGroups())

	const count = 8
	credentials := make(chan *CanvasCredential, count)
	errorsCh := make(chan error, count)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			credential, err := svc.ResolveCanvasCredential(context.Background(), 1, 1, "192.0.2.10")
			credentials <- credential
			errorsCh <- err
		}()
	}
	wg.Wait()
	close(credentials)
	close(errorsCh)

	for err := range errorsCh {
		require.NoError(t, err)
	}
	var key string
	for credential := range credentials {
		require.NotNil(t, credential)
		if key == "" {
			key = credential.APIKey
		}
		assert.Equal(t, key, credential.APIKey)
	}
	assert.Len(t, repo.keys, 1)
}

func TestResolveCanvasCredentialDoesNotReuseAnotherUsersKey(t *testing.T) {
	groupID := int64(1)
	repo := &canvasAPIKeyRepoStub{keys: []APIKey{{ID: 7, UserID: 2, GroupID: &groupID, Key: "sk-other-user", Status: StatusActive, Purpose: APIKeyPurposeGeneral}}}
	svc := newCanvasAPIKeyService(repo, map[int64]*User{1: {ID: 1}, 2: {ID: 2}}, publicCanvasGroups())

	credential, err := svc.ResolveCanvasCredential(context.Background(), 1, groupID, "192.0.2.10")
	require.NoError(t, err)
	assert.NotEqual(t, "sk-other-user", credential.APIKey)
	assert.Len(t, repo.keys, 2)
}

func TestResolveCanvasCredentialRejectsUnavailableGroup(t *testing.T) {
	svc := newCanvasAPIKeyService(&canvasAPIKeyRepoStub{}, map[int64]*User{1: {ID: 1}}, publicCanvasGroups())

	_, err := svc.ResolveCanvasCredential(context.Background(), 1, 3, "192.0.2.10")
	assert.ErrorIs(t, err, ErrGroupNotAllowed)
}

func TestCanvasFeatureSwitchDefaultsOnAndCanBeDisabled(t *testing.T) {
	enabledByDefault := NewSettingService(&settingRepoStub{values: map[string]string{}}, &config.Config{})
	disabled := NewSettingService(&settingRepoStub{values: map[string]string{SettingKeyCanvasEnabled: "false"}}, &config.Config{})

	assert.True(t, enabledByDefault.IsCanvasEnabled(context.Background()))
	assert.False(t, disabled.IsCanvasEnabled(context.Background()))
}
