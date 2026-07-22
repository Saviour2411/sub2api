package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	dbaccountgroup "github.com/Wei-Shaw/sub2api/ent/accountgroup"
	dbgroup "github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/upstreamdailystat"
	"github.com/Wei-Shaw/sub2api/ent/upstreamgroup"
	"github.com/Wei-Shaw/sub2api/ent/upstreamgroupaccountbinding"
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
	if site.SortOrder == 0 {
		last, err := clientFromContext(ctx, r.client).UpstreamSite.Query().
			Order(dbent.Desc(upstreamsite.FieldSortOrder)).
			First(ctx)
		if err != nil && !dbent.IsNotFound(err) {
			return fmt.Errorf("读取上游站点排序: %w", err)
		}
		if last != nil {
			site.SortOrder = last.SortOrder + 10
		}
	}
	b := clientFromContext(ctx, r.client).UpstreamSite.Create().
		SetName(site.Name).
		SetSortOrder(site.SortOrder).
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
	bindingCount, err := clientFromContext(ctx, r.client).UpstreamGroupAccountBinding.Query().
		Where(upstreamgroupaccountbinding.HasUpstreamGroupWith(upstreamgroup.SiteIDEQ(id))).
		Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("统计上游账号绑定: %w", err)
	}
	site := upstreamSiteFromEnt(row)
	site.DisplayedGroupCount = displayedGroupCount
	site.BindingCount = bindingCount
	return site, nil
}

func (r *upstreamRepository) Update(ctx context.Context, site *service.UpstreamSite) error {
	b := clientFromContext(ctx, r.client).UpstreamSite.UpdateOneID(site.ID).
		SetName(site.Name).
		SetSortOrder(site.SortOrder).
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
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("开启删除上游站点事务: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockUpstreamSite(ctx, tx, id); err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrUpstreamNotFound
		}
		return fmt.Errorf("锁定上游站点: %w", err)
	}
	bindings, err := tx.UpstreamGroupAccountBinding.Query().
		Where(upstreamgroupaccountbinding.HasUpstreamGroupWith(upstreamgroup.SiteIDEQ(id))).
		All(ctx)
	if err != nil {
		return fmt.Errorf("查询站点账号绑定: %w", err)
	}
	affectedGroupIDs := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		affectedGroupIDs = append(affectedGroupIDs, binding.LocalGroupID)
	}
	affectedGroupIDs = uniqueSortedPositiveInt64s(affectedGroupIDs)
	if err := lockLocalGroupsByID(ctx, tx.Client(), affectedGroupIDs); err != nil {
		return fmt.Errorf("锁定绑定本地分组: %w", err)
	}
	if err := tx.UpstreamSite.DeleteOneID(id).Exec(ctx); err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrUpstreamNotFound
		}
		return fmt.Errorf("删除上游站点: %w", err)
	}
	if _, err := recalculateUpstreamBindingPriorities(ctx, tx.Client(), affectedGroupIDs); err != nil {
		return fmt.Errorf("重排站点解绑账号: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交删除上游站点事务: %w", err)
	}
	return nil
}

func (r *upstreamRepository) List(ctx context.Context, params service.UpstreamListParams) ([]*service.UpstreamSite, int64, error) {
	q := r.siteQuery(ctx, params)
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
	rows, err := q.Order(upstreamSiteOrder(params)...).Offset((page - 1) * pageSize).Limit(pageSize).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("列出上游站点: %w", err)
	}
	items, err := r.buildSiteList(ctx, rows)
	if err != nil {
		return nil, 0, err
	}
	return items, int64(total), nil
}

func (r *upstreamRepository) ListAll(ctx context.Context, params service.UpstreamListParams) ([]*service.UpstreamSite, error) {
	rows, err := r.siteQuery(ctx, params).Order(upstreamSiteOrder(params)...).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出全部上游站点: %w", err)
	}
	return r.buildSiteList(ctx, rows)
}

func (r *upstreamRepository) siteQuery(ctx context.Context, params service.UpstreamListParams) *dbent.UpstreamSiteQuery {
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
	if groupPlatform := strings.TrimSpace(params.GroupPlatform); groupPlatform != "" {
		q = q.Where(upstreamsite.HasGroupsWith(upstreamgroup.PlatformEqualFold(groupPlatform)))
	}
	return q
}

func upstreamSiteOrder(params service.UpstreamListParams) []upstreamsite.OrderOption {
	if params.SortBy == "balance_usd" {
		if params.SortOrder == "desc" {
			return []upstreamsite.OrderOption{upstreamsite.ByBalanceUsd(entsql.OrderDesc(), entsql.OrderNullsLast()), dbent.Asc(upstreamsite.FieldID)}
		}
		return []upstreamsite.OrderOption{upstreamsite.ByBalanceUsd(entsql.OrderNullsLast()), dbent.Asc(upstreamsite.FieldID)}
	}
	if params.SortBy == "today_tokens" {
		if params.SortOrder == "desc" {
			return []upstreamsite.OrderOption{
				dbent.Desc(upstreamsite.FieldPlatform),
				dbent.Desc(upstreamsite.FieldTodayTokens),
				dbent.Asc(upstreamsite.FieldID),
			}
		}
		return []upstreamsite.OrderOption{
			dbent.Desc(upstreamsite.FieldPlatform),
			dbent.Asc(upstreamsite.FieldTodayTokens),
			dbent.Asc(upstreamsite.FieldID),
		}
	}
	return []upstreamsite.OrderOption{dbent.Asc(upstreamsite.FieldSortOrder), dbent.Asc(upstreamsite.FieldID)}
}

func (r *upstreamRepository) buildSiteList(ctx context.Context, rows []*dbent.UpstreamSite) ([]*service.UpstreamSite, error) {
	displayedCounts := make(map[int64]int, len(rows))
	bindingCounts := make(map[int64]int, len(rows))
	if len(rows) > 0 {
		siteIDs := make([]int64, 0, len(rows))
		for _, row := range rows {
			siteIDs = append(siteIDs, row.ID)
		}
		groups, groupErr := clientFromContext(ctx, r.client).UpstreamGroup.Query().
			Where(upstreamgroup.SiteIDIn(siteIDs...), upstreamgroup.DisplayedEQ(true)).
			All(ctx)
		if groupErr != nil {
			return nil, fmt.Errorf("统计已展示上游分组: %w", groupErr)
		}
		for _, group := range groups {
			displayedCounts[group.SiteID]++
		}
		bindings, bindingErr := clientFromContext(ctx, r.client).UpstreamGroupAccountBinding.Query().
			Where(upstreamgroupaccountbinding.HasUpstreamGroupWith(upstreamgroup.SiteIDIn(siteIDs...))).
			WithUpstreamGroup().
			All(ctx)
		if bindingErr != nil {
			return nil, fmt.Errorf("统计上游账号绑定: %w", bindingErr)
		}
		for _, binding := range bindings {
			if binding.Edges.UpstreamGroup != nil {
				bindingCounts[binding.Edges.UpstreamGroup.SiteID]++
			}
		}
	}
	items := make([]*service.UpstreamSite, 0, len(rows))
	for _, row := range rows {
		item := upstreamSiteFromEnt(row)
		item.DisplayedGroupCount = displayedCounts[row.ID]
		item.BindingCount = bindingCounts[row.ID]
		items = append(items, item)
	}
	return items, nil
}

func (r *upstreamRepository) UpdateSortOrder(ctx context.Context, updates []service.UpstreamSortOrderUpdate) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("开启上游排序事务: %w", err)
	}
	rollback := func(cause error) error {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("%v；回滚失败: %w", cause, rollbackErr)
		}
		return cause
	}
	for _, update := range updates {
		if err := tx.UpstreamSite.UpdateOneID(update.ID).SetSortOrder(update.SortOrder).Exec(ctx); err != nil {
			if dbent.IsNotFound(err) {
				return rollback(service.ErrUpstreamNotFound)
			}
			return rollback(fmt.Errorf("更新上游站点排序: %w", err))
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交上游排序事务: %w", err)
	}
	return nil
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
	if result == nil {
		return service.ErrUpstreamInvalidInput.WithCause(fmt.Errorf("同步结果为空"))
	}
	tokenMetricsAvailable := result.TokenMetricsAvailable == nil || *result.TokenMetricsAvailable
	seenRemoteIDs := make(map[string]struct{}, len(result.Groups))
	for _, group := range result.Groups {
		if strings.TrimSpace(group.RemoteID) == "" {
			return service.ErrUpstreamInvalidInput.WithCause(fmt.Errorf("上游分组 remote_id 为空"))
		}
		if _, exists := seenRemoteIDs[group.RemoteID]; exists {
			return service.ErrUpstreamInvalidInput.WithCause(fmt.Errorf("上游分组 remote_id 重复: %s", group.RemoteID))
		}
		seenRemoteIDs[group.RemoteID] = struct{}{}
		if group.Multiplier != nil {
			if _, ok := normalizeUpstreamBindingMultiplier(*group.Multiplier); !ok {
				return service.ErrUpstreamInvalidInput.WithCause(fmt.Errorf("上游分组 %s 倍率无效", group.RemoteID))
			}
		}
	}
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
	existingGroups, err := tx.UpstreamGroup.Query().Where(upstreamgroup.SiteIDEQ(id)).All(ctx)
	if err != nil {
		return rollback(fmt.Errorf("查询上游分组: %w", err))
	}
	existingByRemoteID := make(map[string]*dbent.UpstreamGroup, len(existingGroups))
	for _, group := range existingGroups {
		existingByRemoteID[group.RemoteID] = group
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
		if exists && equalNormalizedOptionalFloat(existing.Multiplier, group.Multiplier) {
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
	for _, group := range result.Groups {
		if existing := existingByRemoteID[group.RemoteID]; existing != nil {
			update := tx.UpstreamGroup.UpdateOneID(existing.ID).
				SetName(group.Name).
				SetPlatform(group.Platform).
				SetDescription(group.Description).
				SetTodayCostUsd(group.TodayCostUSD).
				SetAvailable(true).
				SetLastSyncedAt(syncedAt)
			if tokenMetricsAvailable {
				update = update.SetTodayTokens(group.TodayTokens)
			}
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
			SetTodayCostUsd(group.TodayCostUSD).
			SetAvailable(true).
			SetLastSyncedAt(syncedAt)
		if tokenMetricsAvailable {
			create = create.SetTodayTokens(group.TodayTokens)
		}
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
	bindings, bindingErr := tx.UpstreamGroupAccountBinding.Query().
		Where(upstreamgroupaccountbinding.HasUpstreamGroupWith(upstreamgroup.SiteIDEQ(id))).
		All(ctx)
	if bindingErr != nil {
		return rollback(fmt.Errorf("查询站点账号绑定: %w", bindingErr))
	}
	affectedLocalGroupIDs := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		affectedLocalGroupIDs = append(affectedLocalGroupIDs, binding.LocalGroupID)
	}
	affectedLocalGroupIDs = uniqueSortedPositiveInt64s(affectedLocalGroupIDs)
	if err := lockLocalGroupsByID(ctx, tx.Client(), affectedLocalGroupIDs); err != nil {
		return rollback(fmt.Errorf("锁定站点绑定本地分组: %w", err))
	}
	if _, err := recalculateUpstreamBindingPriorities(ctx, tx.Client(), affectedLocalGroupIDs); err != nil {
		return rollback(fmt.Errorf("按上游倍率重排账号: %w", err))
	}

	for _, daily := range result.Daily {
		b := tx.UpstreamDailyStat.Create().
			SetSiteID(id).
			SetUsageDate(daily.Date).
			SetCostUsd(daily.CostUSD).
			SetCostBasisVersion(service.UpstreamCostBasisActual)
		if tokenMetricsAvailable {
			b = b.SetTokens(daily.Tokens)
		}
		if daily.BalanceUSD != nil {
			b = b.SetBalanceUsd(*daily.BalanceUSD)
		}
		upsert := b.OnConflictColumns(upstreamdailystat.FieldSiteID, upstreamdailystat.FieldUsageDate)
		if tokenMetricsAvailable {
			upsert = upsert.UpdateNewValues()
		} else {
			upsert = upsert.Update(func(update *dbent.UpstreamDailyStatUpsert) {
				if daily.BalanceUSD != nil {
					update.UpdateBalanceUsd()
				}
				update.UpdateCostUsd().UpdateCostBasisVersion().UpdateUpdatedAt()
			})
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
		SetTodayCostUsd(todayCost).
		SetTotalCostUsd(totalCost).
		SetLastSyncedAt(syncedAt)
	if tokenMetricsAvailable {
		update = update.SetTodayTokens(todayTokens).SetTotalTokens(totalTokens)
	}
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
	site, err := r.GetByID(ctx, siteID)
	if err != nil {
		return nil, err
	}
	tokenMetricsAvailable := service.UpstreamTokenMetricsAvailable(site.Platform)
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
	groupIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		groupIDs = append(groupIDs, row.ID)
	}
	bindingsByGroup, err := loadUpstreamBindingViews(ctx, clientFromContext(ctx, r.client), groupIDs)
	if err != nil {
		return nil, fmt.Errorf("查询上游分组账号绑定: %w", err)
	}
	items := make([]service.UpstreamGroup, 0, len(rows))
	for _, row := range rows {
		item := service.UpstreamGroup{
			ID: row.ID, SiteID: row.SiteID, RemoteID: row.RemoteID, Name: row.Name,
			Platform: row.Platform, Description: row.Description, Multiplier: row.Multiplier, TodayTokens: row.TodayTokens,
			TodayCostUSD: row.TodayCostUsd, Displayed: row.Displayed, Available: row.Available, LastSyncedAt: row.LastSyncedAt,
			TokenMetricsAvailable: tokenMetricsAvailable,
			Bindings:              bindingsByGroup[row.ID],
		}
		if item.Bindings == nil {
			item.Bindings = []service.UpstreamGroupAccountBinding{}
		}
		items = append(items, item)
	}
	return items, nil
}

func loadUpstreamBindingViews(ctx context.Context, client *dbent.Client, upstreamGroupIDs []int64) (map[int64][]service.UpstreamGroupAccountBinding, error) {
	result := make(map[int64][]service.UpstreamGroupAccountBinding, len(upstreamGroupIDs))
	upstreamGroupIDs = uniqueSortedPositiveInt64s(upstreamGroupIDs)
	if len(upstreamGroupIDs) == 0 {
		return result, nil
	}
	rows, err := client.UpstreamGroupAccountBinding.Query().
		Where(upstreamgroupaccountbinding.UpstreamGroupIDIn(upstreamGroupIDs...)).
		Order(upstreamgroupaccountbinding.ByLocalGroupID(), upstreamgroupaccountbinding.ByAccountID()).
		WithLocalGroup().
		WithAccount().
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.Edges.LocalGroup == nil || row.Edges.Account == nil {
			continue
		}
		result[row.UpstreamGroupID] = append(result[row.UpstreamGroupID], service.UpstreamGroupAccountBinding{
			ID: row.ID, UpstreamGroupID: row.UpstreamGroupID,
			LocalGroupID: row.LocalGroupID, LocalGroupName: row.Edges.LocalGroup.Name,
			AccountID: row.AccountID, AccountName: row.Edges.Account.Name,
			AccountPlatform: row.Edges.Account.Platform, AccountStatus: row.Edges.Account.Status,
			AccountPriority: row.Edges.Account.Priority, CreatedAt: row.CreatedAt,
		})
	}
	return result, nil
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
	if !displayed {
		bindingCount, countErr := tx.UpstreamGroupAccountBinding.Query().
			Where(upstreamgroupaccountbinding.UpstreamGroupIDEQ(row.ID)).
			Count(ctx)
		if countErr != nil {
			return rollback(fmt.Errorf("统计上游分组账号绑定: %w", countErr))
		}
		if bindingCount > 0 {
			return rollback(service.ErrUpstreamGroupHasBindings)
		}
	}
	if displayed && !row.Available {
		return rollback(service.ErrUpstreamGroupUnavailable)
	}
	item := upstreamGroupFromEnt(row)
	tokenMetricsAvailable, metricErr := upstreamTokenMetricsAvailableForSite(ctx, tx.Client(), siteID)
	if metricErr != nil {
		return rollback(fmt.Errorf("读取上游站点平台: %w", metricErr))
	}
	item.TokenMetricsAvailable = tokenMetricsAvailable
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
	bindingsByGroup, bindingErr := loadUpstreamBindingViews(ctx, tx.Client(), []int64{row.ID})
	if bindingErr != nil {
		return rollback(fmt.Errorf("读取上游分组账号绑定: %w", bindingErr))
	}
	item.Bindings = bindingsByGroup[row.ID]
	if item.Bindings == nil {
		item.Bindings = []service.UpstreamGroupAccountBinding{}
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交上游分组展示事务: %w", err)
	}
	return &service.UpstreamGroupDisplayResult{Group: item, DisplayedGroupCount: count}, nil
}

func (r *upstreamRepository) ReplaceGroupBindings(
	ctx context.Context,
	siteID, upstreamGroupID int64,
	inputs []service.UpstreamGroupAccountBindingInput,
) (*service.UpstreamGroup, error) {
	requestedAccountIDs := make([]int64, 0, len(inputs))
	for _, input := range inputs {
		requestedAccountIDs = append(requestedAccountIDs, input.AccountID)
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("开启上游账号绑定事务: %w", err)
	}
	rollback := func(cause error) (*service.UpstreamGroup, error) {
		if rbErr := tx.Rollback(); rbErr != nil {
			return nil, fmt.Errorf("%v；回滚失败: %w", cause, rbErr)
		}
		return nil, cause
	}
	if err := lockUpstreamBindingAccountsByID(ctx, tx.Client(), requestedAccountIDs); err != nil {
		return rollback(fmt.Errorf("锁定待绑定账号: %w", err))
	}
	if err := lockUpstreamSite(ctx, tx, siteID); err != nil {
		if dbent.IsNotFound(err) {
			return rollback(service.ErrUpstreamNotFound)
		}
		return rollback(fmt.Errorf("锁定上游站点: %w", err))
	}
	groupRow, err := tx.UpstreamGroup.Query().Where(
		upstreamgroup.IDEQ(upstreamGroupID),
		upstreamgroup.SiteIDEQ(siteID),
	).Only(ctx)
	if dbent.IsNotFound(err) {
		return rollback(service.ErrUpstreamGroupNotFound)
	}
	if err != nil {
		return rollback(fmt.Errorf("查询上游分组: %w", err))
	}
	existingBindings, err := tx.UpstreamGroupAccountBinding.Query().
		Where(upstreamgroupaccountbinding.UpstreamGroupIDEQ(upstreamGroupID)).
		All(ctx)
	if err != nil {
		return rollback(fmt.Errorf("查询现有账号绑定: %w", err))
	}
	existingByAccount := make(map[int64]*dbent.UpstreamGroupAccountBinding, len(existingBindings))
	affectedLocalGroupIDs := make([]int64, 0, len(existingBindings)+len(inputs))
	for _, binding := range existingBindings {
		existingByAccount[binding.AccountID] = binding
		affectedLocalGroupIDs = append(affectedLocalGroupIDs, binding.LocalGroupID)
	}

	accountIDs := make([]int64, 0, len(inputs))
	localGroupIDs := make([]int64, 0, len(inputs))
	desiredByAccount := make(map[int64]service.UpstreamGroupAccountBindingInput, len(inputs))
	hasAddition := false
	for _, input := range inputs {
		accountIDs = append(accountIDs, input.AccountID)
		localGroupIDs = append(localGroupIDs, input.LocalGroupID)
		affectedLocalGroupIDs = append(affectedLocalGroupIDs, input.LocalGroupID)
		desiredByAccount[input.AccountID] = input
		if existing := existingByAccount[input.AccountID]; existing == nil || existing.LocalGroupID != input.LocalGroupID {
			hasAddition = true
		}
	}
	accountIDs = uniqueSortedPositiveInt64s(accountIDs)
	localGroupIDs = uniqueSortedPositiveInt64s(localGroupIDs)
	affectedLocalGroupIDs = uniqueSortedPositiveInt64s(affectedLocalGroupIDs)
	if hasAddition {
		if !groupRow.Displayed || !groupRow.Available || groupRow.Multiplier == nil {
			return rollback(service.ErrUpstreamGroupMultiplierUnavailable)
		}
		if _, ok := normalizeUpstreamBindingMultiplier(*groupRow.Multiplier); !ok {
			return rollback(service.ErrUpstreamGroupMultiplierUnavailable)
		}
	}
	if len(localGroupIDs) > 0 {
		groups, groupErr := tx.Group.Query().Where(dbgroup.IDIn(localGroupIDs...)).All(ctx)
		if groupErr != nil {
			return rollback(fmt.Errorf("查询待绑定本地分组: %w", groupErr))
		}
		if len(groups) != len(localGroupIDs) {
			return rollback(service.ErrGroupNotFound)
		}
	}
	if err := lockLocalGroupsByID(ctx, tx.Client(), affectedLocalGroupIDs); err != nil {
		return rollback(fmt.Errorf("锁定绑定本地分组: %w", err))
	}

	if len(accountIDs) > 0 {
		accounts, accountErr := tx.Account.Query().Where(dbaccount.IDIn(accountIDs...)).All(ctx)
		if accountErr != nil {
			return rollback(fmt.Errorf("查询待绑定账号: %w", accountErr))
		}
		if len(accounts) != len(accountIDs) {
			return rollback(service.ErrAccountNotFound)
		}
		memberships, membershipErr := tx.AccountGroup.Query().Where(
			dbaccountgroup.AccountIDIn(accountIDs...),
			dbaccountgroup.GroupIDIn(localGroupIDs...),
		).All(ctx)
		if membershipErr != nil {
			return rollback(fmt.Errorf("查询账号本地分组关系: %w", membershipErr))
		}
		membershipSet := make(map[[2]int64]struct{}, len(memberships))
		for _, membership := range memberships {
			membershipSet[[2]int64{membership.AccountID, membership.GroupID}] = struct{}{}
		}
		for _, input := range inputs {
			if _, exists := membershipSet[[2]int64{input.AccountID, input.LocalGroupID}]; !exists {
				return rollback(service.ErrUpstreamAccountNotInGroup.WithCause(
					fmt.Errorf("账号 %d 不属于本地分组 %d", input.AccountID, input.LocalGroupID),
				))
			}
		}
		conflict, conflictErr := tx.UpstreamGroupAccountBinding.Query().Where(
			upstreamgroupaccountbinding.AccountIDIn(accountIDs...),
			upstreamgroupaccountbinding.UpstreamGroupIDNEQ(upstreamGroupID),
		).First(ctx)
		if conflictErr != nil && !dbent.IsNotFound(conflictErr) {
			return rollback(fmt.Errorf("检查账号重复绑定: %w", conflictErr))
		}
		if conflict != nil {
			return rollback(service.ErrUpstreamBindingConflict.WithCause(
				fmt.Errorf("账号 %d 已绑定上游分组 %d", conflict.AccountID, conflict.UpstreamGroupID),
			))
		}
	}
	deleteIDs := make([]int64, 0)
	for accountID, existing := range existingByAccount {
		desired, keep := desiredByAccount[accountID]
		if !keep || desired.LocalGroupID != existing.LocalGroupID {
			deleteIDs = append(deleteIDs, existing.ID)
		}
	}
	if len(deleteIDs) > 0 {
		if _, err := tx.UpstreamGroupAccountBinding.Delete().
			Where(upstreamgroupaccountbinding.IDIn(deleteIDs...)).
			Exec(ctx); err != nil {
			return rollback(fmt.Errorf("删除旧账号绑定: %w", err))
		}
	}
	creates := make([]*dbent.UpstreamGroupAccountBindingCreate, 0)
	for _, input := range inputs {
		if existing := existingByAccount[input.AccountID]; existing != nil && existing.LocalGroupID == input.LocalGroupID {
			continue
		}
		creates = append(creates, tx.UpstreamGroupAccountBinding.Create().
			SetUpstreamGroupID(upstreamGroupID).
			SetLocalGroupID(input.LocalGroupID).
			SetAccountID(input.AccountID))
	}
	if len(creates) > 0 {
		if _, err := tx.UpstreamGroupAccountBinding.CreateBulk(creates...).Save(ctx); err != nil {
			return rollback(translatePersistenceError(err, nil, service.ErrUpstreamBindingConflict))
		}
	}
	if _, err := recalculateUpstreamBindingPriorities(ctx, tx.Client(), affectedLocalGroupIDs); err != nil {
		return rollback(fmt.Errorf("重排绑定账号优先级: %w", err))
	}
	bindingsByGroup, err := loadUpstreamBindingViews(ctx, tx.Client(), []int64{upstreamGroupID})
	if err != nil {
		return rollback(fmt.Errorf("读取更新后账号绑定: %w", err))
	}
	item := upstreamGroupFromEnt(groupRow)
	tokenMetricsAvailable, metricErr := upstreamTokenMetricsAvailableForSite(ctx, tx.Client(), siteID)
	if metricErr != nil {
		return rollback(fmt.Errorf("读取上游站点平台: %w", metricErr))
	}
	item.TokenMetricsAvailable = tokenMetricsAvailable
	item.Bindings = bindingsByGroup[upstreamGroupID]
	if item.Bindings == nil {
		item.Bindings = []service.UpstreamGroupAccountBinding{}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交上游账号绑定事务: %w", err)
	}
	return &item, nil
}

func upstreamGroupFromEnt(row *dbent.UpstreamGroup) service.UpstreamGroup {
	return service.UpstreamGroup{
		ID: row.ID, SiteID: row.SiteID, RemoteID: row.RemoteID, Name: row.Name,
		Platform: row.Platform, Description: row.Description, Multiplier: row.Multiplier,
		TodayTokens: row.TodayTokens, TodayCostUSD: row.TodayCostUsd, Displayed: row.Displayed,
		Available: row.Available, LastSyncedAt: row.LastSyncedAt,
	}
}

func upstreamTokenMetricsAvailableForSite(ctx context.Context, client *dbent.Client, siteID int64) (bool, error) {
	site, err := client.UpstreamSite.Query().Where(upstreamsite.IDEQ(siteID)).Only(ctx)
	if err != nil {
		return false, err
	}
	return service.UpstreamTokenMetricsAvailable(string(site.Platform)), nil
}

func lockUpstreamSite(ctx context.Context, tx *dbent.Tx, siteID int64) error {
	_, err := tx.UpstreamSite.Query().Where(upstreamsite.IDEQ(siteID)).ForUpdate().Only(ctx)
	if err != nil && strings.Contains(err.Error(), "not supported in SQLite") {
		_, err = tx.UpstreamSite.Query().Where(upstreamsite.IDEQ(siteID)).Only(ctx)
	}
	return err
}

func (r *upstreamRepository) ListHistory(ctx context.Context, siteID int64, from, through time.Time) ([]service.UpstreamDailyStat, error) {
	site, err := r.GetByID(ctx, siteID)
	if err != nil {
		return nil, err
	}
	tokenMetricsAvailable := service.UpstreamTokenMetricsAvailable(site.Platform)
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
			TokenMetricsAvailable: tokenMetricsAvailable,
			CreatedAt:             row.CreatedAt, UpdatedAt: row.UpdatedAt,
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

func upstreamSiteFromEnt(row *dbent.UpstreamSite) *service.UpstreamSite {
	site := &service.UpstreamSite{}
	assignUpstreamSite(site, row)
	return site
}

func assignUpstreamSite(site *service.UpstreamSite, row *dbent.UpstreamSite) {
	displayedGroupCount := site.DisplayedGroupCount
	bindingCount := site.BindingCount
	*site = service.UpstreamSite{
		ID: row.ID, SortOrder: row.SortOrder, Name: row.Name, BaseURL: row.BaseURL, Platform: string(row.Platform),
		AuthMode: string(row.AuthMode), Account: row.Account, CredentialEncrypted: row.CredentialEncrypted,
		Enabled: row.Enabled, Status: string(row.Status), ErrorMessage: row.ErrorMessage,
		BalanceUSD: row.BalanceUsd, TodayTokens: row.TodayTokens, TodayCostUSD: row.TodayCostUsd,
		TotalTokens: row.TotalTokens, TotalCostUSD: row.TotalCostUsd,
		TrackingStartedAt: row.TrackingStartedAt, LastSyncedAt: row.LastSyncedAt,
		NextSyncAt: row.NextSyncAt, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		DisplayedGroupCount: displayedGroupCount, BindingCount: bindingCount,
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
