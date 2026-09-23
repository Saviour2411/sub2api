//go:build integration

package repository

import (
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

func (s *UsageLogRepoSuite) TestUsageExportCursorSurvivesConcurrentInsert() {
	user := mustCreateUser(s.T(), s.client, &service.User{Email: "export-cursor@example.com"})
	key := mustCreateApiKey(s.T(), s.client, &service.APIKey{UserID: user.ID, Key: "sk-export-cursor", Name: "export"})
	account := mustCreateAccount(s.T(), s.client, &service.Account{Name: "export"})
	create := func(model string) int64 {
		row := &service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID,
			RequestID: uuid.NewString(), Model: model, CreatedAt: time.Now()}
		_, err := s.repo.Create(s.ctx, row)
		s.Require().NoError(err)
		return row.ID
	}
	first, second, third := create("match"), create("match"), create("match")
	create("other")
	repo := s.repo
	filters := usagestats.UsageLogFilters{UserID: user.ID, Model: "match"}
	rows, next, err := repo.ListExport(s.ctx, usagestats.ExportPageOptions{PageSize: 2}, filters)
	s.Require().NoError(err)
	s.Require().Len(rows, 2)
	s.Require().Equal([]int64{third, second}, []int64{rows[0].ID, rows[1].ID})
	s.Require().NotNil(rows[0].APIKey)
	create("match")
	before, err := strconv.ParseInt(next, 10, 64)
	s.Require().NoError(err)
	rows, next, err = repo.ListExport(s.ctx, usagestats.ExportPageOptions{BeforeID: before, PageSize: 2}, filters)
	s.Require().NoError(err)
	s.Require().Len(rows, 1)
	s.Require().Equal(first, rows[0].ID)
	s.Require().Empty(next)
	filters.UserID = user.ID + 1000000
	rows, next, err = repo.ListExport(s.ctx, usagestats.ExportPageOptions{PageSize: 2}, filters)
	s.Require().NoError(err)
	s.Require().Empty(rows)
	s.Require().Empty(next)
}
