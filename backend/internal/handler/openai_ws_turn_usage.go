package handler

import (
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type openAIWSTurnUsageContext struct {
	requestedModel     string
	channelMapping     service.ChannelMappingResult
	requestPayloadHash string
}

type openAIWSTurnUsageTracker struct {
	mu    sync.Mutex
	turns map[int]openAIWSTurnUsageContext
}

func newOpenAIWSTurnUsageTracker() *openAIWSTurnUsageTracker {
	return &openAIWSTurnUsageTracker{turns: make(map[int]openAIWSTurnUsageContext, 2)}
}

func (t *openAIWSTurnUsageTracker) Store(turn int, requestedModel string, mapping service.ChannelMappingResult, payloadHash string) {
	if t == nil || turn <= 0 {
		return
	}
	t.mu.Lock()
	t.turns[turn] = openAIWSTurnUsageContext{
		requestedModel:     strings.TrimSpace(requestedModel),
		channelMapping:     mapping,
		requestPayloadHash: payloadHash,
	}
	t.mu.Unlock()
}

func (t *openAIWSTurnUsageTracker) Take(turn int) (openAIWSTurnUsageContext, bool) {
	if t == nil || turn <= 0 {
		return openAIWSTurnUsageContext{}, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	usageContext, ok := t.turns[turn]
	delete(t.turns, turn)
	return usageContext, ok
}
