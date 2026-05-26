package service

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type RedeemCode struct {
	ID        int64
	Code      string
	Type      string
	Value     float64
	Status    string
	MaxUses   int
	UsedCount int
	UsedBy    *int64
	UsedAt    *time.Time
	Notes     string
	CreatedAt time.Time
	ExpiresAt *time.Time

	GroupID      *int64
	ValidityDays int

	User  *User
	Group *Group
}

func (r *RedeemCode) IsUsed() bool {
	return r.Status == StatusUsed
}

func (r *RedeemCode) IsExpired() bool {
	return r.IsExpiredAt(time.Now())
}

func (r *RedeemCode) IsExpiredAt(now time.Time) bool {
	if r == nil {
		return false
	}
	if r.Status == StatusExpired {
		return true
	}
	return r.Status == StatusUnused && r.ExpiresAt != nil && !r.ExpiresAt.After(now)
}

func (r *RedeemCode) CanUse() bool {
	if r.Status != StatusUnused || r.IsExpired() {
		return false
	}
	maxUses := r.MaxUses
	if maxUses <= 0 {
		maxUses = 1
	}
	return r.UsedCount < maxUses
}

func (r *RedeemCode) RemainingUses() int {
	if r == nil {
		return 0
	}
	maxUses := r.MaxUses
	if maxUses <= 0 {
		maxUses = 1
	}
	remaining := maxUses - r.UsedCount
	if remaining < 0 {
		return 0
	}
	return remaining
}

type RedeemCodeUsage struct {
	ID           int64
	RedeemCodeID int64
	UserID       int64
	Type         string
	Value        float64
	GroupID      *int64
	ValidityDays int
	UsedAt       time.Time

	User       *User
	RedeemCode *RedeemCode
	Group      *Group
}

func GenerateRedeemCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
