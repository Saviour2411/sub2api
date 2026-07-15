package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/upstreamdailystat"
	"github.com/Wei-Shaw/sub2api/ent/upstreamgroup"
	"github.com/Wei-Shaw/sub2api/ent/upstreamgroupmultiplierhistory"
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
	displayedGroupCount, err := clientFromContext(ctx, r.client).UpstreamGroup.Query().
		Where(upstreamgroup.SiteIDEQ(id), upstreamgroup.DisplayedEQ(true)).
		Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("统计已展示上游分组: %w", err)
	}
	site := upstreamSiteFromEnt(row)
	site.DisplayedGroupCount = displayedGroupCount
	return site, nil
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
	displayedCounts := make(map[int64]int, len(rows))
	if len(rows) > 0 {
		siteIDs := make([]int64, 0, len(rows))
		for _, row := range rows {
			siteIDs = append(siteIDs, row.ID)
		}
		groups, groupErr := clientFromContext(ctx, r.client).UpstreamGroup.Query().
			Where(upstreamgroup.SiteIDIn(siteIDs...), upstreamgroup.DisplayedEQ(true)).
			All(ctx)
		if groupErr != nil {
			return nil, 0, fmt.Errorf("统计已展示上游分组: %w", groupErr)
		}
		for _, group := range groups {
			displayedCounts[group.SiteID]++
		}
	}
	items := make([]*service.UpstreamSite, 0, len(rows))
	for _, row := range rows {
		item := upstreamSiteFromEnt(row)
		item.DisplayedGroupCount = displayedCounts[row.ID]
		items = append(items, item)
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

func (r *upstreamRepository) UpdateCredential(ctx context.Context, id int64, encryptedCredential string) error {
	if err := clientFromContext(ctx, r.client).UpstreamSite.UpdateOneID(id).
		SetCredentialEncrypted(encryptedCredential).
		Exec(ctx); err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrUpstreamNotFound
		}
		return fmt.Errorf("更新上游凭证: %w", err)
	}
	return nil
}

func (r *upstreamRepository) MissingDates(ctx context.Context, id int64, from, through time.Time, loc *time.Location) ([]time.Time, error) {
	if loc == nil {
		loc = time.Local
	}
	start := dayStart(from, loc)
	end := dayStart(through, loc)
	earliest := end.AddDate(0, 0, -365)
	if start.Before(earliest) {
		start = earliest
	}
	rows, err := clientFromContext(ctx, r.client).UpstreamDailyStat.Query().
		Where(
			upstreamdailystat.SiteIDEQ(id),
			upstreamdailystat.UsageDateGTE(start),
			upstreamdailystat.UsageDateLTE(end),
			upstreamdailystat.CostBasisVersionGTE(service.UpstreamCostBasisActual),
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

	if err = lockUpstreamSite(ctx, tx, id); err != nil {
		if dbent.IsNotFound(err) {
			return rollback(service.ErrUpstreamNotFound)
		}
		return rollback(fmt.Errorf("锁定上游站点: %w", err))
	}
	historyRows, err := tx.UpstreamGroupMultiplierHistory.Query().
		Where(upstreamgroupmultiplierhistory.SiteIDEQ(id)).
		Order(
			dbent.Desc(upstreamgroupmultiplierhistory.FieldRecordedAt),
			dbent.Desc(upstreamgroupmultiplierhistory.FieldID),
		).
		All(ctx)
	if err != nil {
		return rollback(fmt.Errorf("查询上游分组最新倍率: %w", err))
	}
	latestHistoryByRemoteID := make(map[string]*dbent.UpstreamGroupMultiplierHistory, len(historyRows))
	for _, row := range historyRows {
		if _, exists := latestHistoryByRemoteID[row.RemoteID]; !exists {
			latestHistoryByRemoteID[row.RemoteID] = row
		}
	}
	for _, group := range result.Groups {
		existing, exists := latestHistoryByRemoteID[group.RemoteID]
		if exists && equalOptionalFloat(existing.Multiplier, group.Multiplier) {
			continue
		}
		builder := tx.UpstreamGroupMultiplierHistory.Create().
			SetSiteID(id).
			SetRemoteID(group.RemoteID).
			SetName(group.Name).
			SetPlatform(group.Platform).
			SetDescription(group.Description).
			SetRecordedAt(syncedAt)
		if group.Multiplier != nil {
			builder = builder.SetMultiplier(*group.Multiplier)
		}
		if _, err = builder.Save(ctx); err != nil {
			return rollback(fmt.Errorf("记录上游分组倍率: %w", err))
		}
	}
	if err = tx.UpstreamGroup.Update().Where(upstreamgroup.SiteIDEQ(id)).SetAvailable(false).Exec(ctx); err != nil {
		return rollback(fmt.Errorf("标记上游分组不可用: %w", err))
	}
	existingGroups, err := tx.UpstreamGroup.Query().Where(upstreamgroup.SiteIDEQ(id)).All(ctx)
	if err != nil {
		return rollback(fmt.Errorf("查询上游分组: %w", err))
	}
	existingByRemoteID := make(map[string]*dbent.UpstreamGroup, len(existingGroups))
	for _, group := range existingGroups {
		existingByRemoteID[group.RemoteID] = group
	}
	for _, group := range result.Groups {
		if existing := existingByRemoteID[group.RemoteID]; existing != nil {
			update := tx.UpstreamGroup.UpdateOneID(existing.ID).
				SetName(group.Name).
				SetPlatform(group.Platform).
				SetDescription(group.Description).
				SetTodayTokens(group.TodayTokens).
				SetTodayCostUsd(group.TodayCostUSD).
				SetAvailable(true).
				SetLastSyncedAt(syncedAt)
			if group.Multiplier != nil {
				update = update.SetMultiplier(*group.Multiplier)
			} else {
				update = update.ClearMultiplier()
			}
			if err = update.Exec(ctx); err != nil {
				return rollback(fmt.Errorf("更新上游分组: %w", err))
			}
			continue
		}
		create := tx.UpstreamGroup.Create().
			SetSiteID(id).
			SetRemoteID(group.RemoteID).
			SetName(group.Name).
			SetPlatform(group.Platform).
			SetDescription(group.Description).
			SetTodayTokens(group.TodayTokens).
			SetTodayCostUsd(group.TodayCostUSD).
			SetAvailable(true).
			SetLastSyncedAt(syncedAt)
		if group.Multiplier != nil {
			create = create.SetMultiplier(*group.Multiplier)
		}
		created, createErr := create.Save(ctx)
		if createErr != nil {
			return rollback(fmt.Errorf("保存上游分组: %w", createErr))
		}
		existingByRemoteID[group.RemoteID] = created
	}
	if _, err = tx.UpstreamGroup.Delete().Where(
		upstreamgroup.SiteIDEQ(id),
		upstreamgroup.AvailableEQ(false),
		upstreamgroup.DisplayedEQ(false),
	).Exec(ctx); err != nil {
		return rollback(fmt.Errorf("清理失效上游分组: %w", err))
	}

	for _, daily := range result.Daily {
		b := tx.UpstreamDailyStat.Create().
			SetSiteID(id).
			SetUsageDate(daily.Date).
			SetTokens(daily.Tokens).
			SetCostUsd(daily.CostUSD).
			SetCostBasisVersion(service.UpstreamCostBasisActual)
		if daily.BalanceUSD != nil {
			b = b.SetBalanceUsd(*daily.BalanceUSD)
		}
		upsert := b.OnConflictColumns(upstreamdailystat.FieldSiteID, upstreamdailystat.FieldUsageDate).UpdateNewValues()
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
		if stat.CostBasisVersion >= service.UpstreamCostBasisActual {
			totalCost += stat.CostUsd
		}
		if dayStart(stat.UsageDate, shanghaiLocation()).Equal(today) {
			todayTokens = stat.Tokens
			if stat.CostBasisVersion >= service.UpstreamCostBasisActual {
				todayCost = stat.CostUsd
			}
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
		Where(
			upstreamgroup.SiteIDEQ(siteID),
			upstreamgroup.Or(upstreamgroup.AvailableEQ(true), upstreamgroup.DisplayedEQ(true)),
		).
		Order(dbent.Desc(upstreamgroup.FieldAvailable), dbent.Asc(upstreamgroup.FieldName)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询上游分组: %w", err)
	}
	items := make([]service.UpstreamGroup, 0, len(rows))
	for _, row := range rows {
		items = append(items, service.UpstreamGroup{
			ID: row.ID, SiteID: row.SiteID, RemoteID: row.RemoteID, Name: row.Name,
			Platform: row.Platform, Description: row.Description, Multiplier: row.Multiplier, TodayTokens: row.TodayTokens,
			TodayCostUSD: row.TodayCostUsd, Displayed: row.Displayed, Available: row.Available, LastSyncedAt: row.LastSyncedAt,
		})
	}
	return items, nil
}

func (r *upstreamRepository) SetGroupDisplayed(ctx context.Context, siteID int64, remoteID string, displayed bool) (*service.UpstreamGroupDisplayResult, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("开启上游分组展示事务: %w", err)
	}
	rollback := func(cause error) (*service.UpstreamGroupDisplayResult, error) {
		if rbErr := tx.Rollback(); rbErr != nil {
			return nil, fmt.Errorf("%v；回滚失败: %w", cause, rbErr)
		}
		return nil, cause
	}
	if err = lockUpstreamSite(ctx, tx, siteID); err != nil {
		if dbent.IsNotFound(err) {
			return rollback(service.ErrUpstreamNotFound)
		}
		return rollback(fmt.Errorf("锁定上游站点: %w", err))
	}
	row, err := tx.UpstreamGroup.Query().Where(
		upstreamgroup.SiteIDEQ(siteID),
		upstreamgroup.RemoteIDEQ(remoteID),
	).Only(ctx)
	if dbent.IsNotFound(err) {
		return rollback(service.ErrUpstreamGroupNotFound)
	}
	if err != nil {
		return rollback(fmt.Errorf("查询上游分组: %w", err))
	}
	if displayed && !row.Available {
		return rollback(service.ErrUpstreamGroupUnavailable)
	}
	item := upstreamGroupFromEnt(row)
	item.Displayed = displayed
	if !displayed && !row.Available {
		if err = tx.UpstreamGroup.DeleteOneID(row.ID).Exec(ctx); err != nil {
			return rollback(fmt.Errorf("删除失效上游分组: %w", err))
		}
	} else if err = tx.UpstreamGroup.UpdateOneID(row.ID).SetDisplayed(displayed).Exec(ctx); err != nil {
		return rollback(fmt.Errorf("更新上游分组展示状态: %w", err))
	}
	count, err := tx.UpstreamGroup.Query().Where(
		upstreamgroup.SiteIDEQ(siteID),
		upstreamgroup.DisplayedEQ(true),
	).Count(ctx)
	if err != nil {
		return rollback(fmt.Errorf("统计已展示上游分组: %w", err))
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交上游分组展示事务: %w", err)
	}
	return &service.UpstreamGroupDisplayResult{Group: item, DisplayedGroupCount: count}, nil
}

func upstreamGroupFromEnt(row *dbent.UpstreamGroup) service.UpstreamGroup {
	return service.UpstreamGroup{
		ID: row.ID, SiteID: row.SiteID, RemoteID: row.RemoteID, Name: row.Name,
		Platform: row.Platform, Description: row.Description, Multiplier: row.Multiplier,
		TodayTokens: row.TodayTokens, TodayCostUSD: row.TodayCostUsd, Displayed: row.Displayed,
		Available: row.Available, LastSyncedAt: row.LastSyncedAt,
	}
}

func lockUpstreamSite(ctx context.Context, tx *dbent.Tx, siteID int64) error {
	_, err := tx.UpstreamSite.Query().Where(upstreamsite.IDEQ(siteID)).ForUpdate().Only(ctx)
	if err != nil && strings.Contains(err.Error(), "not supported in SQLite") {
		_, err = tx.UpstreamSite.Query().Where(upstreamsite.IDEQ(siteID)).Only(ctx)
	}
	return err
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
			upstreamdailystat.CostBasisVersionGTE(service.UpstreamCostBasisActual),
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
			Tokens: row.Tokens, CostUSD: row.CostUsd, CostBasisVersion: row.CostBasisVersion,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return items, nil
}

func (r *upstreamRepository) ListMultiplierHistory(ctx context.Context, siteID int64, from, through time.Time) ([]service.UpstreamGroupMultiplierHistory, error) {
	if _, err := r.GetByID(ctx, siteID); err != nil {
		return nil, err
	}
	groups, err := clientFromContext(ctx, r.client).UpstreamGroup.Query().
		Where(upstreamgroup.SiteIDEQ(siteID)).
		Order(dbent.Asc(upstreamgroup.FieldName), dbent.Asc(upstreamgroup.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询上游分组: %w", err)
	}
	rows, err := clientFromContext(ctx, r.client).UpstreamGroupMultiplierHistory.Query().
		Where(
			upstreamgroupmultiplierhistory.SiteIDEQ(siteID),
			upstreamgroupmultiplierhistory.RecordedAtLTE(through),
		).
		Order(
			dbent.Asc(upstreamgroupmultiplierhistory.FieldRecordedAt),
			dbent.Asc(upstreamgroupmultiplierhistory.FieldID),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询上游分组倍率历史: %w", err)
	}

	type historyGroup struct {
		current bool
		item    service.UpstreamGroupMultiplierHistory
		rows    []*dbent.UpstreamGroupMultiplierHistory
	}
	byRemoteID := make(map[string]*historyGroup, len(groups))
	for _, group := range groups {
		byRemoteID[group.RemoteID] = &historyGroup{
			current: true,
			item: service.UpstreamGroupMultiplierHistory{
				RemoteID:          group.RemoteID,
				Name:              group.Name,
				Platform:          group.Platform,
				Description:       group.Description,
				CurrentMultiplier: group.Multiplier,
			},
		}
	}
	for _, row := range rows {
		group := byRemoteID[row.RemoteID]
		if group == nil {
			group = &historyGroup{item: service.UpstreamGroupMultiplierHistory{RemoteID: row.RemoteID}}
			byRemoteID[row.RemoteID] = group
		}
		group.rows = append(group.rows, row)
		if !group.current {
			group.item.Name = row.Name
			group.item.Platform = row.Platform
			group.item.Description = row.Description
			group.item.CurrentMultiplier = row.Multiplier
		}
	}
	items := make([]service.UpstreamGroupMultiplierHistory, 0, len(byRemoteID))
	for _, group := range byRemoteID {
		group.item.Points = multiplierHistoryPoints(group.rows, from, through)
		items = append(items, group.item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].RemoteID < items[j].RemoteID
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func multiplierHistoryPoints(rows []*dbent.UpstreamGroupMultiplierHistory, from, through time.Time) []service.UpstreamGroupMultiplierPoint {
	points := make([]service.UpstreamGroupMultiplierPoint, 0, len(rows)+2)
	var prior *dbent.UpstreamGroupMultiplierHistory
	for _, row := range rows {
		if row.RecordedAt.Before(from) {
			prior = row
			continue
		}
		points = append(points, service.UpstreamGroupMultiplierPoint{
			RecordedAt: row.RecordedAt,
			Multiplier: row.Multiplier,
		})
	}
	if prior != nil && (len(points) == 0 || !points[0].RecordedAt.Equal(from)) {
		points = append([]service.UpstreamGroupMultiplierPoint{{
			RecordedAt: from,
			Multiplier: prior.Multiplier,
		}}, points...)
	}
	if len(points) > 0 && points[len(points)-1].RecordedAt.Before(through) {
		points = append(points, service.UpstreamGroupMultiplierPoint{
			RecordedAt: through,
			Multiplier: points[len(points)-1].Multiplier,
		})
	}
	return points
}

func equalOptionalFloat(left, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
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
