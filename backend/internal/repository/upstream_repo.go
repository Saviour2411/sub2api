package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/upstreamdailystat"
	"github.com/Wei-Shaw/sub2api/ent/upstreamgroup"
	"github.com/Wei-Shaw/sub2api/ent/upstreamsite"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type upstreamRepository struct {
	client *dbent.Client
}

func NewUpstreamRepository(client *dbent.Client) service.UpstreamRepository {
	return &upstreamRepository{client: client}
}

func (r *upstreamRepository) Create(ctx context.Context, site *service.UpstreamSite) error {
	b := clientFromContext(ctx, r.client).UpstreamSite.Create().
		SetName(site.Name).
		SetBaseURL(site.BaseURL).
		SetPlatform(upstreamsite.Platform(site.Platform)).
		SetAuthMode(upstreamsite.AuthMode(site.AuthMode)).
		SetAccount(site.Account).
		SetCredentialEncrypted(site.CredentialEncrypted).
		SetEnabled(site.Enabled).
		SetStatus(upstreamsite.Status(site.Status)).
		SetTrackingStartedAt(site.TrackingStartedAt).
		SetCreatedBy(site.CreatedBy)
	if site.NextSyncAt != nil {
		b = b.SetNextSyncAt(*site.NextSyncAt)
	}
	row, err := b.Save(ctx)
	if err != nil {
		return fmt.Errorf("创建上游站点: %w", err)
	}
	assignUpstreamSite(site, row)
	return nil
}

func (r *upstreamRepository) GetByID(ctx context.Context, id int64) (*service.UpstreamSite, error) {
	row, err := clientFromContext(ctx, r.client).UpstreamSite.Query().Where(upstreamsite.IDEQ(id)).Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, service.ErrUpstreamNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询上游站点: %w", err)
	}
	return upstreamSiteFromEnt(row), nil
}

func (r *upstreamRepository) Update(ctx context.Context, site *service.UpstreamSite) error {
	b := clientFromContext(ctx, r.client).UpstreamSite.UpdateOneID(site.ID).
		SetName(site.Name).
		SetBaseURL(site.BaseURL).
		SetPlatform(upstreamsite.Platform(site.Platform)).
		SetAuthMode(upstreamsite.AuthMode(site.AuthMode)).
		SetAccount(site.Account).
		SetCredentialEncrypted(site.CredentialEncrypted).
		SetEnabled(site.Enabled).
		SetStatus(upstreamsite.Status(site.Status)).
		ClearErrorMessage()
	if site.NextSyncAt != nil {
		b = b.SetNextSyncAt(*site.NextSyncAt)
	} else {
		b = b.ClearNextSyncAt()
	}
	row, err := b.Save(ctx)
	if dbent.IsNotFound(err) {
		return service.ErrUpstreamNotFound
	}
	if err != nil {
		return fmt.Errorf("更新上游站点: %w", err)
	}
	assignUpstreamSite(site, row)
	return nil
}

func (r *upstreamRepository) SetEnabled(ctx context.Context, id int64, enabled bool, nextSyncAt *time.Time) error {
	b := clientFromContext(ctx, r.client).UpstreamSite.UpdateOneID(id).SetEnabled(enabled)
	if nextSyncAt != nil {
		b = b.SetNextSyncAt(*nextSyncAt)
	} else {
		b = b.ClearNextSyncAt()
	}
	if err := b.Exec(ctx); err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrUpstreamNotFound
		}
		return fmt.Errorf("更新上游启用状态: %w", err)
	}
	return nil
}

func (r *upstreamRepository) Delete(ctx context.Context, id int64) error {
	err := clientFromContext(ctx, r.client).UpstreamSite.DeleteOneID(id).Exec(ctx)
	if dbent.IsNotFound(err) {
		return service.ErrUpstreamNotFound
	}
	if err != nil {
		return fmt.Errorf("删除上游站点: %w", err)
	}
	return nil
}

func (r *upstreamRepository) List(ctx context.Context, params service.UpstreamListParams) ([]*service.UpstreamSite, int64, error) {
	q := clientFromContext(ctx, r.client).UpstreamSite.Query()
	if search := strings.TrimSpace(params.Search); search != "" {
		q = q.Where(upstreamsite.Or(
			upstreamsite.NameContainsFold(search),
			upstreamsite.BaseURLContainsFold(search),
			upstreamsite.AccountContainsFold(search),
		))
	}
	if params.Platform != "" {
		q = q.Where(upstreamsite.PlatformEQ(upstreamsite.Platform(params.Platform)))
	}
	if params.Enabled != nil {
		q = q.Where(upstreamsite.EnabledEQ(*params.Enabled))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("统计上游站点: %w", err)
	}
	page, pageSize := params.Page, params.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	rows, err := q.Order(dbent.Desc(upstreamsite.FieldID)).Offset((page - 1) * pageSize).Limit(pageSize).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("列出上游站点: %w", err)
	}
	items := make([]*service.UpstreamSite, 0, len(rows))
	for _, row := range rows {
		items = append(items, upstreamSiteFromEnt(row))
	}
	return items, int64(total), nil
}

func (r *upstreamRepository) ListIDs(ctx context.Context, enabledOnly bool) ([]int64, error) {
	q := clientFromContext(ctx, r.client).UpstreamSite.Query()
	if enabledOnly {
		q = q.Where(upstreamsite.EnabledEQ(true))
	}
	ids, err := q.IDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出上游站点 ID: %w", err)
	}
	return ids, nil
}

func (r *upstreamRepository) ListDue(ctx context.Context, now time.Time, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 100
	}
	ids, err := clientFromContext(ctx, r.client).UpstreamSite.Query().
		Where(
			upstreamsite.EnabledEQ(true),
			upstreamsite.Or(upstreamsite.NextSyncAtIsNil(), upstreamsite.NextSyncAtLTE(now)),
		).
		Order(dbent.Asc(upstreamsite.FieldNextSyncAt)).
		Limit(limit).
		IDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询待同步上游站点: %w", err)
	}
	return ids, nil
}

func (r *upstreamRepository) MarkPending(ctx context.Context, id int64, nextSyncAt *time.Time) error {
	b := clientFromContext(ctx, r.client).UpstreamSite.UpdateOneID(id).
		SetStatus(upstreamsite.StatusPending).
		ClearErrorMessage()
	if nextSyncAt != nil {
		b = b.SetNextSyncAt(*nextSyncAt)
	} else {
		b = b.ClearNextSyncAt()
	}
	if err := b.Exec(ctx); err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrUpstreamNotFound
		}
		return fmt.Errorf("标记上游待同步: %w", err)
	}
	return nil
}

func (r *upstreamRepository) MarkSyncing(ctx context.Context, id int64) error {
	if err := clientFromContext(ctx, r.client).UpstreamSite.UpdateOneID(id).
		SetStatus(upstreamsite.StatusSyncing).
		ClearErrorMessage().
		Exec(ctx); err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrUpstreamNotFound
		}
		return fmt.Errorf("标记上游同步中: %w", err)
	}
	return nil
}

func (r *upstreamRepository) MarkSyncFailed(ctx context.Context, id int64, message string, nextSyncAt *time.Time) error {
	if len(message) > 500 {
		message = message[:500]
	}
	b := clientFromContext(ctx, r.client).UpstreamSite.UpdateOneID(id).
		SetStatus(upstreamsite.StatusError).
		SetErrorMessage(message)
	if nextSyncAt != nil {
		b = b.SetNextSyncAt(*nextSyncAt)
	} else {
		b = b.ClearNextSyncAt()
	}
	if err := b.Exec(ctx); err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrUpstreamNotFound
		}
		return fmt.Errorf("记录上游同步错误: %w", err)
	}
	return nil
}

func (r *upstreamRepository) MissingDates(ctx context.Context, id int64, from, through time.Time, loc *time.Location) ([]time.Time, error) {
	if loc == nil {
		loc = time.Local
	}
	start := dayStart(from, loc)
	end := dayStart(through, loc)
	rows, err := clientFromContext(ctx, r.client).UpstreamDailyStat.Query().
		Where(
			upstreamdailystat.SiteIDEQ(id),
			upstreamdailystat.UsageDateGTE(start),
			upstreamdailystat.UsageDateLTE(end),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询上游历史日期: %w", err)
	}
	existing := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		existing[row.UsageDate.In(loc).Format("2006-01-02")] = struct{}{}
	}
	result := make([]time.Time, 0)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		if d.Equal(end) {
			result = append(result, d)
			continue
		}
		if _, ok := existing[key]; !ok {
			result = append(result, d)
		}
	}
	return result, nil
}

func (r *upstreamRepository) CommitSync(ctx context.Context, id int64, result *service.UpstreamSyncResult, encryptedCredential string, syncedAt time.Time, nextSyncAt *time.Time) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("开启上游同步事务: %w", err)
	}
	rollback := func(cause error) error {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("%v；回滚失败: %w", cause, rbErr)
		}
		return cause
	}

	if _, err = tx.UpstreamSite.Query().Where(upstreamsite.IDEQ(id)).Only(ctx); err != nil {
		if dbent.IsNotFound(err) {
			return rollback(service.ErrUpstreamNotFound)
		}
		return rollback(fmt.Errorf("锁定上游站点: %w", err))
	}
	if _, err = tx.UpstreamGroup.Delete().Where(upstreamgroup.SiteIDEQ(id)).Exec(ctx); err != nil {
		return rollback(fmt.Errorf("替换上游分组: %w", err))
	}
	if len(result.Groups) > 0 {
		builders := make([]*dbent.UpstreamGroupCreate, 0, len(result.Groups))
		for _, group := range result.Groups {
			b := tx.UpstreamGroup.Create().
				SetSiteID(id).
				SetRemoteID(group.RemoteID).
				SetName(group.Name).
				SetPlatform(group.Platform).
				SetTodayTokens(group.TodayTokens).
				SetTodayCostUsd(group.TodayCostUSD).
				SetLastSyncedAt(syncedAt)
			if group.Multiplier != nil {
				b = b.SetMultiplier(*group.Multiplier)
			}
			builders = append(builders, b)
		}
		if _, err = tx.UpstreamGroup.CreateBulk(builders...).Save(ctx); err != nil {
			return rollback(fmt.Errorf("保存上游分组: %w", err))
		}
	}

	for _, daily := range result.Daily {
		b := tx.UpstreamDailyStat.Create().
			SetSiteID(id).
			SetUsageDate(daily.Date).
			SetTokens(daily.Tokens).
			SetCostUsd(daily.CostUSD)
		if daily.BalanceUSD != nil {
			b = b.SetBalanceUsd(*daily.BalanceUSD)
		}
		upsert := b.OnConflictColumns(upstreamdailystat.FieldSiteID, upstreamdailystat.FieldUsageDate).UpdateNewValues()
		if daily.BalanceUSD == nil {
			upsert = upsert.ClearBalanceUsd()
		}
		if err = upsert.Exec(ctx); err != nil {
			return rollback(fmt.Errorf("保存上游每日统计: %w", err))
		}
	}

	stats, err := tx.UpstreamDailyStat.Query().Where(upstreamdailystat.SiteIDEQ(id)).All(ctx)
	if err != nil {
		return rollback(fmt.Errorf("汇总上游历史: %w", err))
	}
	var totalTokens int64
	var totalCost float64
	var todayTokens int64
	var todayCost float64
	today := dayStart(syncedAt, shanghaiLocation())
	for _, stat := range stats {
		totalTokens += stat.Tokens
		totalCost += stat.CostUsd
		if dayStart(stat.UsageDate, shanghaiLocation()).Equal(today) {
			todayTokens = stat.Tokens
			todayCost = stat.CostUsd
		}
	}

	update := tx.UpstreamSite.UpdateOneID(id).
		SetStatus(upstreamsite.StatusHealthy).
		ClearErrorMessage().
		SetTodayTokens(todayTokens).
		SetTodayCostUsd(todayCost).
		SetTotalTokens(totalTokens).
		SetTotalCostUsd(totalCost).
		SetLastSyncedAt(syncedAt)
	if result.BalanceUSD != nil {
		update = update.SetBalanceUsd(*result.BalanceUSD)
	} else {
		update = update.ClearBalanceUsd()
	}
	if encryptedCredential != "" {
		update = update.SetCredentialEncrypted(encryptedCredential)
	}
	if nextSyncAt != nil {
		update = update.SetNextSyncAt(*nextSyncAt)
	} else {
		update = update.ClearNextSyncAt()
	}
	if err = update.Exec(ctx); err != nil {
		return rollback(fmt.Errorf("提交上游同步结果: %w", err))
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交上游同步事务: %w", err)
	}
	return nil
}

func (r *upstreamRepository) ListGroups(ctx context.Context, siteID int64) ([]service.UpstreamGroup, error) {
	if _, err := r.GetByID(ctx, siteID); err != nil {
		return nil, err
	}
	rows, err := clientFromContext(ctx, r.client).UpstreamGroup.Query().
		Where(upstreamgroup.SiteIDEQ(siteID)).
		Order(dbent.Asc(upstreamgroup.FieldName)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询上游分组: %w", err)
	}
	items := make([]service.UpstreamGroup, 0, len(rows))
	for _, row := range rows {
		items = append(items, service.UpstreamGroup{
			ID: row.ID, SiteID: row.SiteID, RemoteID: row.RemoteID, Name: row.Name,
			Platform: row.Platform, Multiplier: row.Multiplier, TodayTokens: row.TodayTokens,
			TodayCostUSD: row.TodayCostUsd, LastSyncedAt: row.LastSyncedAt,
		})
	}
	return items, nil
}

func (r *upstreamRepository) ListHistory(ctx context.Context, siteID int64, from, through time.Time) ([]service.UpstreamDailyStat, error) {
	if _, err := r.GetByID(ctx, siteID); err != nil {
		return nil, err
	}
	rows, err := clientFromContext(ctx, r.client).UpstreamDailyStat.Query().
		Where(
			upstreamdailystat.SiteIDEQ(siteID),
			upstreamdailystat.UsageDateGTE(from),
			upstreamdailystat.UsageDateLTE(through),
		).
		Order(dbent.Asc(upstreamdailystat.FieldUsageDate)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询上游历史: %w", err)
	}
	items := make([]service.UpstreamDailyStat, 0, len(rows))
	for _, row := range rows {
		items = append(items, service.UpstreamDailyStat{
			ID: row.ID, SiteID: row.SiteID, Date: row.UsageDate, BalanceUSD: row.BalanceUsd,
			Tokens: row.Tokens, CostUSD: row.CostUsd, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return items, nil
}

func upstreamSiteFromEnt(row *dbent.UpstreamSite) *service.UpstreamSite {
	site := &service.UpstreamSite{}
	assignUpstreamSite(site, row)
	return site
}

func assignUpstreamSite(site *service.UpstreamSite, row *dbent.UpstreamSite) {
	*site = service.UpstreamSite{
		ID: row.ID, Name: row.Name, BaseURL: row.BaseURL, Platform: string(row.Platform),
		AuthMode: string(row.AuthMode), Account: row.Account, CredentialEncrypted: row.CredentialEncrypted,
		Enabled: row.Enabled, Status: string(row.Status), ErrorMessage: row.ErrorMessage,
		BalanceUSD: row.BalanceUsd, TodayTokens: row.TodayTokens, TodayCostUSD: row.TodayCostUsd,
		TotalTokens: row.TotalTokens, TotalCostUSD: row.TotalCostUsd,
		TrackingStartedAt: row.TrackingStartedAt, LastSyncedAt: row.LastSyncedAt,
		NextSyncAt: row.NextSyncAt, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func dayStart(value time.Time, loc *time.Location) time.Time {
	value = value.In(loc)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, loc)
}

func shanghaiLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return loc
}
