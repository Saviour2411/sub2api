package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const upstreamBindingMultiplierScale = 1_000_000

type upstreamBindingPriorityRow struct {
	localGroupID    int64
	accountID       int64
	multiplier      float64
	accountPriority int
}

// normalizeUpstreamBindingMultiplier 为变化检测和同倍率分组提供稳定精度。
func normalizeUpstreamBindingMultiplier(value float64) (float64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0, false
	}
	normalized := math.Round(value*upstreamBindingMultiplierScale) / upstreamBindingMultiplierScale
	if math.IsNaN(normalized) || math.IsInf(normalized, 0) {
		return 0, false
	}
	return normalized, true
}

func equalNormalizedOptionalFloat(left, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	leftValue, leftOK := normalizeUpstreamBindingMultiplier(*left)
	rightValue, rightOK := normalizeUpstreamBindingMultiplier(*right)
	return leftOK && rightOK && leftValue == rightValue
}

func lockUpstreamSitesByID(ctx context.Context, client *dbent.Client, siteIDs []int64) error {
	return lockRowsByID(ctx, client, "upstream_sites", siteIDs, false)
}

func lockLocalGroupsByID(ctx context.Context, client *dbent.Client, groupIDs []int64) error {
	return lockRowsByID(ctx, client, "groups", groupIDs, true)
}

// lockUpstreamBindingAccountsByID 串行化账号删除与新增上游绑定。
// 它使用独立的事务级 advisory lock，不会阻塞同步事务对 accounts.priority 的更新。
func lockUpstreamBindingAccountsByID(ctx context.Context, client *dbent.Client, accountIDs []int64) error {
	accountIDs = uniqueSortedPositiveInt64s(accountIDs)
	if len(accountIDs) == 0 || client.Driver().Dialect() != dialect.Postgres {
		return nil
	}
	rows, err := client.QueryContext(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended('upstream_binding_account:' || account_id::text, 0))
		FROM unnest($1::bigint[]) AS account_ids(account_id)
		ORDER BY account_id`, pq.Array(accountIDs))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var ignored any
		if err := rows.Scan(&ignored); err != nil {
			return err
		}
	}
	return rows.Err()
}

func lockRowsByID(ctx context.Context, client *dbent.Client, table string, ids []int64, softDeleted bool) error {
	ids = uniqueSortedPositiveInt64s(ids)
	if len(ids) == 0 {
		return nil
	}
	placeholders := postgresPlaceholders(1, len(ids))
	query := fmt.Sprintf("SELECT id FROM %s WHERE id IN (%s)", table, strings.Join(placeholders, ", "))
	if softDeleted {
		query += " AND deleted_at IS NULL"
	}
	query += " ORDER BY id"
	if client.Driver().Dialect() == dialect.Postgres {
		query += " FOR UPDATE"
	}
	rows, err := client.QueryContext(ctx, query, int64AnySlice(ids)...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	count := 0
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count != len(ids) {
		return fmt.Errorf("锁定 %s 失败：目标记录不存在", table)
	}
	return nil
}

// recalculateUpstreamBindingPriorities 必须在已锁定 local group 的事务中调用。
// 任一绑定的上游分组不可用或倍率为空时，整个本地分组冻结，不产生部分重排。
func recalculateUpstreamBindingPriorities(ctx context.Context, client *dbent.Client, localGroupIDs []int64) ([]int64, error) {
	localGroupIDs = uniqueSortedPositiveInt64s(localGroupIDs)
	if len(localGroupIDs) == 0 {
		return nil, nil
	}
	query := `
		SELECT b.local_group_id, b.account_id, ug.available, ug.multiplier, a.priority
		FROM upstream_group_account_bindings b
		JOIN upstream_groups ug ON ug.id = b.upstream_group_id
		JOIN accounts a ON a.id = b.account_id AND a.deleted_at IS NULL
		WHERE b.local_group_id IN (` + strings.Join(postgresPlaceholders(1, len(localGroupIDs)), ", ") + `)
		ORDER BY b.local_group_id ASC, b.account_id ASC`
	rows, err := client.QueryContext(ctx, query, int64AnySlice(localGroupIDs)...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	rowsByGroup := make(map[int64][]upstreamBindingPriorityRow, len(localGroupIDs))
	frozenGroups := make(map[int64]struct{})
	for rows.Next() {
		var (
			localGroupID int64
			accountID    int64
			available    bool
			multiplier   sql.NullFloat64
			priority     int
		)
		if err := rows.Scan(&localGroupID, &accountID, &available, &multiplier, &priority); err != nil {
			return nil, err
		}
		if !available || !multiplier.Valid {
			frozenGroups[localGroupID] = struct{}{}
			continue
		}
		normalized, ok := normalizeUpstreamBindingMultiplier(multiplier.Float64)
		if !ok {
			return nil, fmt.Errorf("本地分组 %d 的绑定倍率无效", localGroupID)
		}
		rowsByGroup[localGroupID] = append(rowsByGroup[localGroupID], upstreamBindingPriorityRow{
			localGroupID: localGroupID, accountID: accountID, multiplier: normalized, accountPriority: priority,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	priorityByAccount := make(map[int64]int)
	for _, groupID := range localGroupIDs {
		if _, frozen := frozenGroups[groupID]; frozen {
			continue
		}
		groupRows := rowsByGroup[groupID]
		sort.Slice(groupRows, func(i, j int) bool {
			if groupRows[i].multiplier != groupRows[j].multiplier {
				return groupRows[i].multiplier < groupRows[j].multiplier
			}
			return groupRows[i].accountID < groupRows[j].accountID
		})
		priority := 10
		var previousMultiplier float64
		for index, row := range groupRows {
			if index > 0 && row.multiplier != previousMultiplier {
				priority += 5
			}
			previousMultiplier = row.multiplier
			if row.accountPriority != priority {
				priorityByAccount[row.accountID] = priority
			}
		}
	}
	if len(priorityByAccount) == 0 {
		return nil, nil
	}

	accountIDs := make([]int64, 0, len(priorityByAccount))
	for accountID := range priorityByAccount {
		accountIDs = append(accountIDs, accountID)
	}
	sort.Slice(accountIDs, func(i, j int) bool { return accountIDs[i] < accountIDs[j] })
	args := make([]any, 0, len(accountIDs)*2)
	caseClauses := make([]string, 0, len(accountIDs))
	idPlaceholders := make([]string, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		idPlaceholder := fmt.Sprintf("$%d", len(args)+1)
		args = append(args, accountID)
		priorityPlaceholder := fmt.Sprintf("$%d", len(args)+1)
		args = append(args, priorityByAccount[accountID])
		caseClauses = append(caseClauses, "WHEN "+idPlaceholder+" THEN "+priorityPlaceholder)
		idPlaceholders = append(idPlaceholders, idPlaceholder)
	}
	updateQuery := `UPDATE accounts
		SET priority = CASE id ` + strings.Join(caseClauses, " ") + ` ELSE priority END,
			updated_at = CURRENT_TIMESTAMP
		WHERE id IN (` + strings.Join(idPlaceholders, ", ") + `) AND deleted_at IS NULL`
	if _, err := client.ExecContext(ctx, updateQuery, args...); err != nil {
		return nil, err
	}
	payload := map[string]any{"account_ids": accountIDs, "group_ids": localGroupIDs}
	if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventAccountBulkChanged, nil, nil, payload); err != nil {
		return nil, err
	}
	return accountIDs, nil
}

func upstreamBindingScopesForAccount(ctx context.Context, client *dbent.Client, accountID int64, localGroupIDs []int64) ([]int64, []int64, error) {
	args := []any{accountID}
	query := `
		SELECT DISTINCT ug.site_id, b.local_group_id
		FROM upstream_group_account_bindings b
		JOIN upstream_groups ug ON ug.id = b.upstream_group_id
		WHERE b.account_id = $1`
	if len(localGroupIDs) > 0 {
		localGroupIDs = uniqueSortedPositiveInt64s(localGroupIDs)
		query += " AND b.local_group_id IN (" + strings.Join(postgresPlaceholders(2, len(localGroupIDs)), ", ") + ")"
		args = append(args, int64AnySlice(localGroupIDs)...)
	}
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	var siteIDs []int64
	var groupIDs []int64
	for rows.Next() {
		var siteID, groupID int64
		if err := rows.Scan(&siteID, &groupID); err != nil {
			return nil, nil, err
		}
		siteIDs = append(siteIDs, siteID)
		groupIDs = append(groupIDs, groupID)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return uniqueSortedPositiveInt64s(siteIDs), uniqueSortedPositiveInt64s(groupIDs), nil
}

// removeUpstreamBindingsForAccount 删除账号的指定绑定并重排剩余绑定；解绑账号自身优先级保持不变。
func removeUpstreamBindingsForAccount(ctx context.Context, client *dbent.Client, accountID int64, localGroupIDs []int64) error {
	requestedGroupIDs := uniqueSortedPositiveInt64s(localGroupIDs)
	siteIDs, affectedGroupIDs, err := upstreamBindingScopesForAccount(ctx, client, accountID, localGroupIDs)
	if err != nil {
		return err
	}
	lockGroupIDs := uniqueSortedPositiveInt64s(append(affectedGroupIDs, requestedGroupIDs...))
	if len(lockGroupIDs) == 0 {
		return nil
	}
	if err := lockUpstreamSitesByID(ctx, client, siteIDs); err != nil {
		return err
	}
	if err := lockLocalGroupsByID(ctx, client, lockGroupIDs); err != nil {
		return err
	}
	// 锁后重读，覆盖“首次查询尚未绑定、并发绑定随后提交”的窗口。
	_, affectedGroupIDs, err = upstreamBindingScopesForAccount(ctx, client, accountID, localGroupIDs)
	if err != nil || len(affectedGroupIDs) == 0 {
		return err
	}
	args := []any{accountID}
	query := "DELETE FROM upstream_group_account_bindings WHERE account_id = $1"
	if len(localGroupIDs) > 0 {
		localGroupIDs = uniqueSortedPositiveInt64s(localGroupIDs)
		query += " AND local_group_id IN (" + strings.Join(postgresPlaceholders(2, len(localGroupIDs)), ", ") + ")"
		args = append(args, int64AnySlice(localGroupIDs)...)
	}
	if _, err := client.ExecContext(ctx, query, args...); err != nil {
		return err
	}
	_, err = recalculateUpstreamBindingPriorities(ctx, client, affectedGroupIDs)
	return err
}

func uniqueSortedPositiveInt64s(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func postgresPlaceholders(start, count int) []string {
	placeholders := make([]string, count)
	for index := 0; index < count; index++ {
		placeholders[index] = fmt.Sprintf("$%d", start+index)
	}
	return placeholders
}

func int64AnySlice(values []int64) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}
