package service

import (
	"context"
	"strings"
)

const DefaultScheduledTestPrompt = "hi"

func (s *SettingService) GetScheduledTestDefaultPrompt(ctx context.Context) string {
	if s == nil || s.settingRepo == nil {
		return DefaultScheduledTestPrompt
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyScheduledTestDefaultPrompt)
	if err != nil {
		return DefaultScheduledTestPrompt
	}
	return normalizeScheduledTestDefaultPrompt(value)
}

func normalizeScheduledTestDefaultPrompt(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return DefaultScheduledTestPrompt
	}
	return value
}
