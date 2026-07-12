package handler

import "sync"

type openAIWSTurnReleasePair struct {
	user    func()
	account func()
}

// openAIWSTurnReleaseTracker 按 turn 隔离并发槽位，避免相邻 turn 的回调交错时
// 上一轮 AfterTurn 误释放下一轮刚获取的槽位。
type openAIWSTurnReleaseTracker struct {
	mu    sync.Mutex
	turns map[int]openAIWSTurnReleasePair
}

func newOpenAIWSTurnReleaseTracker() *openAIWSTurnReleaseTracker {
	return &openAIWSTurnReleaseTracker{turns: make(map[int]openAIWSTurnReleasePair)}
}

func (t *openAIWSTurnReleaseTracker) setUser(turn int, release func()) {
	if t == nil || turn <= 0 || release == nil {
		return
	}
	t.mu.Lock()
	pair := t.turns[turn]
	previous := pair.user
	pair.user = release
	t.turns[turn] = pair
	t.mu.Unlock()
	if previous != nil {
		previous()
	}
}

func (t *openAIWSTurnReleaseTracker) setAccount(turn int, release func()) {
	if t == nil || turn <= 0 || release == nil {
		return
	}
	t.mu.Lock()
	pair := t.turns[turn]
	previous := pair.account
	pair.account = release
	t.turns[turn] = pair
	t.mu.Unlock()
	if previous != nil {
		previous()
	}
}

func (t *openAIWSTurnReleaseTracker) setTurn(turn int, userRelease, accountRelease func()) {
	if t == nil || turn <= 0 {
		return
	}
	t.mu.Lock()
	previous := t.turns[turn]
	t.turns[turn] = openAIWSTurnReleasePair{user: userRelease, account: accountRelease}
	t.mu.Unlock()
	if previous.account != nil {
		previous.account()
	}
	if previous.user != nil {
		previous.user()
	}
}

func (t *openAIWSTurnReleaseTracker) hasUser(turn int) bool {
	if t == nil || turn <= 0 {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.turns[turn].user != nil
}

func (t *openAIWSTurnReleaseTracker) releaseAccount(turn int) {
	if t == nil || turn <= 0 {
		return
	}
	t.mu.Lock()
	pair := t.turns[turn]
	release := pair.account
	pair.account = nil
	if pair.user == nil {
		delete(t.turns, turn)
	} else {
		t.turns[turn] = pair
	}
	t.mu.Unlock()
	if release != nil {
		release()
	}
}

func (t *openAIWSTurnReleaseTracker) releaseTurn(turn int) {
	if t == nil || turn <= 0 {
		return
	}
	t.mu.Lock()
	pair := t.turns[turn]
	delete(t.turns, turn)
	t.mu.Unlock()
	if pair.account != nil {
		pair.account()
	}
	if pair.user != nil {
		pair.user()
	}
}

func (t *openAIWSTurnReleaseTracker) releaseAll() {
	if t == nil {
		return
	}
	t.mu.Lock()
	pairs := make([]openAIWSTurnReleasePair, 0, len(t.turns))
	for turn, pair := range t.turns {
		pairs = append(pairs, pair)
		delete(t.turns, turn)
	}
	t.mu.Unlock()
	for _, pair := range pairs {
		if pair.account != nil {
			pair.account()
		}
		if pair.user != nil {
			pair.user()
		}
	}
}
