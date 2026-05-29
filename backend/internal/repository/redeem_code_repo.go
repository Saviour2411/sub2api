package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"

	entsql "entgo.io/ent/dialect/sql"
)

type redeemCodeRepository struct {
	client *dbent.Client
}

func NewRedeemCodeRepository(client *dbent.Client) service.RedeemCodeRepository {
	return &redeemCodeRepository{client: client}
}

func (r *redeemCodeRepository) Create(ctx context.Context, code *service.RedeemCode) error {
	client := clientFromContext(ctx, r.client)
	created, err := client.RedeemCode.Create().
		SetCode(code.Code).
		SetType(code.Type).
		SetValue(code.Value).
		SetStatus(code.Status).
		SetNotes(code.Notes).
		SetValidityDays(code.ValidityDays).
		SetNillableExpiresAt(code.ExpiresAt).
		SetNillableUsedBy(code.UsedBy).
		SetNillableUsedAt(code.UsedAt).
		SetNillableGroupID(code.GroupID).
		Save(ctx)
	if err == nil {
		code.ID = created.ID
		code.CreatedAt = created.CreatedAt
		code.MaxUses = normalizeRedeemMaxUses(code.MaxUses)
		if err := r.updateUsageLimits(ctx, code.ID, code.MaxUses, code.UsedCount); err != nil {
			if isUndefinedColumn(err) {
				return nil
			}
			return err
		}
	}
	return err
}

func (r *redeemCodeRepository) CreateBatch(ctx context.Context, codes []service.RedeemCode) error {
	if len(codes) == 0 {
		return nil
	}

	client := clientFromContext(ctx, r.client)
	builders := make([]*dbent.RedeemCodeCreate, 0, len(codes))
	for i := range codes {
		c := &codes[i]
		b := client.RedeemCode.Create().
			SetCode(c.Code).
			SetType(c.Type).
			SetValue(c.Value).
			SetStatus(c.Status).
			SetNotes(c.Notes).
			SetValidityDays(c.ValidityDays).
			SetNillableExpiresAt(c.ExpiresAt).
			SetNillableUsedBy(c.UsedBy).
			SetNillableUsedAt(c.UsedAt).
			SetNillableGroupID(c.GroupID)
		builders = append(builders, b)
	}

	created, err := client.RedeemCode.CreateBulk(builders...).Save(ctx)
	if err != nil {
		return err
	}

	for i := range created {
		codes[i].ID = created[i].ID
		codes[i].CreatedAt = created[i].CreatedAt
		codes[i].MaxUses = normalizeRedeemMaxUses(codes[i].MaxUses)
		if err := r.updateUsageLimits(ctx, created[i].ID, codes[i].MaxUses, codes[i].UsedCount); err != nil {
			if isUndefinedColumn(err) {
				return nil
			}
			return err
		}
	}
	return nil
}

func (r *redeemCodeRepository) GetByID(ctx context.Context, id int64) (*service.RedeemCode, error) {
	m, err := r.client.RedeemCode.Query().
		Where(redeemcode.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrRedeemCodeNotFound
		}
		return nil, err
	}
	out := redeemCodeEntityToService(m)
	if err := r.fillUsageLimits(ctx, []*service.RedeemCode{out}); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *redeemCodeRepository) GetByCode(ctx context.Context, code string) (*service.RedeemCode, error) {
	m, err := r.client.RedeemCode.Query().
		Where(redeemcode.CodeEQ(code)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrRedeemCodeNotFound
		}
		return nil, err
	}
	out := redeemCodeEntityToService(m)
	if err := r.fillUsageLimits(ctx, []*service.RedeemCode{out}); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *redeemCodeRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.client.RedeemCode.Delete().Where(redeemcode.IDEQ(id)).Exec(ctx)
	return err
}

func (r *redeemCodeRepository) List(ctx context.Context, params pagination.PaginationParams) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	return r.ListWithFilters(ctx, params, "", "", "")
}

func (r *redeemCodeRepository) ListWithFilters(ctx context.Context, params pagination.PaginationParams, codeType, status, search string) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	q := r.client.RedeemCode.Query()

	if codeType != "" {
		q = q.Where(redeemcode.TypeEQ(codeType))
	}
	if status != "" {
		now := time.Now()
		switch status {
		case service.StatusExpired:
			q = q.Where(redeemcode.Or(
				redeemcode.StatusEQ(service.StatusExpired),
				redeemcode.And(
					redeemcode.StatusEQ(service.StatusUnused),
					redeemcode.ExpiresAtNotNil(),
					redeemcode.ExpiresAtLTE(now),
				),
			))
		case service.StatusUnused:
			q = q.Where(
				redeemcode.StatusEQ(service.StatusUnused),
				redeemcode.Or(
					redeemcode.ExpiresAtIsNil(),
					redeemcode.ExpiresAtGT(now),
				),
			)
		default:
			q = q.Where(redeemcode.StatusEQ(status))
		}
	}
	if search != "" {
		q = q.Where(
			redeemcode.Or(
				redeemcode.CodeContainsFold(search),
				redeemcode.HasUserWith(user.EmailContainsFold(search)),
			),
		)
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	codesQuery := q.
		WithUser().
		WithGroup().
		Offset(params.Offset()).
		Limit(params.Limit())
	for _, order := range redeemCodeListOrder(params) {
		codesQuery = codesQuery.Order(order)
	}

	codes, err := codesQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}

	outCodes := redeemCodeEntitiesToService(codes)
	if err := r.fillUsageLimitsForValues(ctx, outCodes); err != nil {
		return nil, nil, err
	}

	return outCodes, paginationResultFromTotal(int64(total), params), nil
}

func redeemCodeListOrder(params pagination.PaginationParams) []func(*entsql.Selector) {
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	sortOrder := params.NormalizedSortOrder(pagination.SortOrderDesc)

	var field string
	switch sortBy {
	case "type":
		field = redeemcode.FieldType
	case "value":
		field = redeemcode.FieldValue
	case "status":
		field = redeemcode.FieldStatus
	case "used_at":
		field = redeemcode.FieldUsedAt
	case "created_at":
		field = redeemcode.FieldCreatedAt
	case "expires_at":
		field = redeemcode.FieldExpiresAt
	case "code":
		field = redeemcode.FieldCode
	default:
		field = redeemcode.FieldID
	}

	if sortOrder == pagination.SortOrderAsc {
		return []func(*entsql.Selector){dbent.Asc(field), dbent.Asc(redeemcode.FieldID)}
	}
	return []func(*entsql.Selector){dbent.Desc(field), dbent.Desc(redeemcode.FieldID)}
}

func (r *redeemCodeRepository) Update(ctx context.Context, code *service.RedeemCode) error {
	up := r.client.RedeemCode.UpdateOneID(code.ID).
		SetCode(code.Code).
		SetType(code.Type).
		SetValue(code.Value).
		SetStatus(code.Status).
		SetNotes(code.Notes).
		SetValidityDays(code.ValidityDays)

	if code.UsedBy != nil {
		up.SetUsedBy(*code.UsedBy)
	} else {
		up.ClearUsedBy()
	}
	if code.UsedAt != nil {
		up.SetUsedAt(*code.UsedAt)
	} else {
		up.ClearUsedAt()
	}
	if code.GroupID != nil {
		up.SetGroupID(*code.GroupID)
	} else {
		up.ClearGroupID()
	}
	if code.ExpiresAt != nil {
		up.SetExpiresAt(*code.ExpiresAt)
	} else {
		up.ClearExpiresAt()
	}

	updated, err := up.Save(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrRedeemCodeNotFound
		}
		return err
	}
	code.CreatedAt = updated.CreatedAt
	code.MaxUses = normalizeRedeemMaxUses(code.MaxUses)
	if err := r.updateUsageLimits(ctx, code.ID, code.MaxUses, code.UsedCount); err != nil {
		if isUndefinedColumn(err) {
			return nil
		}
		return err
	}
	return nil
}

func (r *redeemCodeRepository) BatchUpdate(ctx context.Context, ids []int64, fields service.RedeemCodeBatchUpdateFields) (int64, error) {
	uniqueIDs := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	if len(uniqueIDs) == 0 {
		return 0, nil
	}

	if tx := dbent.TxFromContext(ctx); tx != nil {
		return r.batchUpdate(ctx, tx.Client(), uniqueIDs, fields)
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return 0, err
	}
	txCtx := dbent.NewTxContext(ctx, tx)
	defer func() { _ = tx.Rollback() }()

	updated, err := r.batchUpdate(txCtx, tx.Client(), uniqueIDs, fields)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return updated, nil
}

func (r *redeemCodeRepository) batchUpdate(ctx context.Context, client *dbent.Client, ids []int64, fields service.RedeemCodeBatchUpdateFields) (int64, error) {
	existing, err := client.RedeemCode.Query().
		Where(redeemcode.IDIn(ids...)).
		All(ctx)
	if err != nil {
		return 0, err
	}
	if len(existing) != len(ids) {
		return 0, service.ErrRedeemCodeNotFound
	}
	if fields.TouchesUsedSensitiveFields() {
		for _, code := range existing {
			if code.Status == service.StatusUsed {
				return 0, service.ErrRedeemCodeUsed
			}
		}
	}

	up := client.RedeemCode.Update().Where(redeemcode.IDIn(ids...))
	if fields.Status != nil {
		up.SetStatus(*fields.Status)
	}
	if fields.Notes != nil {
		up.SetNotes(*fields.Notes)
	}
	if fields.ExpiresAt.Set {
		if fields.ExpiresAt.Value != nil {
			up.SetExpiresAt(*fields.ExpiresAt.Value)
		} else {
			up.ClearExpiresAt()
		}
	}
	if fields.GroupID.Set {
		if fields.GroupID.Value != nil {
			up.SetGroupID(*fields.GroupID.Value)
		} else {
			up.ClearGroupID()
		}
	}

	affected, err := up.Save(ctx)
	if err != nil {
		return 0, err
	}
	if affected != len(ids) {
		return 0, service.ErrRedeemCodeNotFound
	}
	return int64(affected), nil
}

func (r *redeemCodeRepository) Use(ctx context.Context, id, userID int64) error {
	now := time.Now()
	client := clientFromContext(ctx, r.client)
	var candidateCount, insertedCount, updatedCount int
	err := scanSingleRow(ctx, client, `
WITH candidate AS (
    SELECT id, type, value, group_id, validity_days
    FROM redeem_codes
    WHERE id = $1
      AND status = $5
      AND used_count < max_uses
    FOR UPDATE
),
inserted AS (
INSERT INTO redeem_code_usages (
    redeem_code_id,
    user_id,
    type,
    value,
    group_id,
    validity_days,
    used_at
) SELECT id, $2, type, value, group_id, validity_days, $3
  FROM candidate
ON CONFLICT (redeem_code_id, user_id) DO NOTHING
  RETURNING redeem_code_id
),
updated AS (
UPDATE redeem_codes
SET used_count = used_count + 1,
    used_by = $2,
    used_at = $3,
    status = CASE WHEN used_count + 1 >= max_uses THEN $4 ELSE status END
WHERE id IN (SELECT redeem_code_id FROM inserted)
  RETURNING id
)
SELECT
    (SELECT COUNT(*) FROM candidate),
    (SELECT COUNT(*) FROM inserted),
    (SELECT COUNT(*) FROM updated)
`, []any{id, userID, now, service.StatusUsed, service.StatusUnused}, &candidateCount, &insertedCount, &updatedCount)
	if err != nil {
		if isUndefinedColumn(err) || isMissingRedeemUsageTable(err) {
			return r.useLegacy(ctx, id, userID, now)
		}
		return err
	}
	if candidateCount == 0 {
		return service.ErrRedeemCodeUsed
	}
	if insertedCount == 0 {
		return service.ErrRedeemCodeUsedByUser
	}
	if updatedCount == 0 {
		return service.ErrRedeemCodeUsed
	}
	return nil
}

func (r *redeemCodeRepository) useLegacy(ctx context.Context, id, userID int64, usedAt time.Time) error {
	client := clientFromContext(ctx, r.client)
	affected, err := client.RedeemCode.Update().
		Where(redeemcode.IDEQ(id), redeemcode.StatusEQ(service.StatusUnused)).
		SetStatus(service.StatusUsed).
		SetUsedBy(userID).
		SetUsedAt(usedAt).
		Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrRedeemCodeUsed
	}
	return nil
}

func (r *redeemCodeRepository) ListByUser(ctx context.Context, userID int64, limit int) ([]service.RedeemCode, error) {
	if limit <= 0 {
		limit = 10
	}

	codes, _, err := r.listUsageHistoryByUser(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: limit}, "")
	if err == nil {
		return codes, nil
	}
	if !isMissingRedeemUsageTable(err) {
		return nil, err
	}

	entities, err := r.client.RedeemCode.Query().
		Where(redeemcode.UsedByEQ(userID)).
		WithGroup().
		Order(dbent.Desc(redeemcode.FieldUsedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := redeemCodeEntitiesToService(entities)
	if err := r.fillUsageLimitsForValues(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListByUserPaginated returns paginated balance/concurrency history for a user.
// Supports optional type filter (e.g. "balance", "admin_balance", "concurrency", "admin_concurrency", "subscription").
func (r *redeemCodeRepository) ListByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams, codeType string) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	codes, result, err := r.listUsageHistoryByUser(ctx, userID, params, codeType)
	if err == nil {
		return codes, result, nil
	}
	if !isMissingRedeemUsageTable(err) {
		return nil, nil, err
	}

	q := r.client.RedeemCode.Query().
		Where(redeemcode.UsedByEQ(userID))

	// Optional type filter
	if codeType != "" {
		q = q.Where(redeemcode.TypeEQ(codeType))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	entities, err := q.
		WithGroup().
		Offset(params.Offset()).
		Limit(params.Limit()).
		Order(dbent.Desc(redeemcode.FieldUsedAt)).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}

	out := redeemCodeEntitiesToService(entities)
	if err := r.fillUsageLimitsForValues(ctx, out); err != nil {
		return nil, nil, err
	}
	return out, paginationResultFromTotal(int64(total), params), nil
}

// SumPositiveBalanceByUser returns total recharged amount (sum of value > 0 where type is balance/admin_balance).
func (r *redeemCodeRepository) SumPositiveBalanceByUser(ctx context.Context, userID int64) (float64, error) {
	total, err := r.sumPositiveBalanceUsageByUser(ctx, userID)
	if err == nil {
		return total, nil
	}
	if !isMissingRedeemUsageTable(err) {
		return 0, err
	}

	var result []struct {
		Sum float64 `json:"sum"`
	}
	err = r.client.RedeemCode.Query().
		Where(
			redeemcode.UsedByEQ(userID),
			redeemcode.ValueGT(0),
			redeemcode.TypeIn("balance", "admin_balance"),
		).
		Aggregate(dbent.As(dbent.Sum(redeemcode.FieldValue), "sum")).
		Scan(ctx, &result)
	if err != nil {
		return 0, err
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0].Sum, nil
}

func (r *redeemCodeRepository) listUsageHistoryByUser(ctx context.Context, userID int64, params pagination.PaginationParams, codeType string) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	client := clientFromContext(ctx, r.client)
	filterSQL := "WHERE rcu.user_id = $1"
	args := []any{userID}
	if codeType != "" {
		filterSQL += " AND rcu.type = $2"
		args = append(args, codeType)
	}

	countRows, err := client.QueryContext(ctx, "SELECT COUNT(*) FROM redeem_code_usages rcu "+filterSQL, args...)
	if err != nil {
		return nil, nil, err
	}
	var total int64
	if countRows.Next() {
		if err := countRows.Scan(&total); err != nil {
			_ = countRows.Close()
			return nil, nil, err
		}
	}
	if err := countRows.Close(); err != nil {
		return nil, nil, err
	}

	queryArgs := append([]any{}, args...)
	limitArg := len(queryArgs) + 1
	offsetArg := len(queryArgs) + 2
	queryArgs = append(queryArgs, params.Limit(), params.Offset())
	rows, err := client.QueryContext(ctx, fmt.Sprintf(`
SELECT rcu.redeem_code_id,
       rc.code,
       rcu.type,
       rcu.value,
       rc.status,
       rc.max_uses,
       rc.used_count,
       rcu.user_id,
       rcu.used_at,
       rc.created_at,
       rc.expires_at,
       rcu.group_id,
       rcu.validity_days,
       rc.notes,
       g.id,
       g.name,
       g.platform,
       g.rate_multiplier,
       g.subscription_type
FROM redeem_code_usages rcu
JOIN redeem_codes rc ON rc.id = rcu.redeem_code_id
LEFT JOIN groups g ON g.id = rcu.group_id
%s
ORDER BY rcu.used_at DESC, rcu.id DESC
LIMIT $%d OFFSET $%d
`, filterSQL, limitArg, offsetArg), queryArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	codes := make([]service.RedeemCode, 0, params.Limit())
	for rows.Next() {
		code, err := scanRedeemUsageHistoryRow(rows)
		if err != nil {
			return nil, nil, err
		}
		codes = append(codes, code)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return codes, paginationResultFromTotal(total, params), nil
}

func (r *redeemCodeRepository) sumPositiveBalanceUsageByUser(ctx context.Context, userID int64) (float64, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT COALESCE(SUM(value), 0)
FROM redeem_code_usages
WHERE user_id = $1
  AND value > 0
  AND type IN ('balance', 'admin_balance')
`, userID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var total float64
	if err := rows.Scan(&total); err != nil {
		return 0, err
	}
	return total, rows.Err()
}

func scanRedeemUsageHistoryRow(rows *sql.Rows) (service.RedeemCode, error) {
	var (
		code              service.RedeemCode
		usedBy            int64
		usedAt            time.Time
		expiresAt         sql.NullTime
		groupID           sql.NullInt64
		notes             sql.NullString
		groupRowID        sql.NullInt64
		groupName         sql.NullString
		groupPlatform     sql.NullString
		groupRate         sql.NullFloat64
		groupSubscription sql.NullString
	)
	if err := rows.Scan(
		&code.ID,
		&code.Code,
		&code.Type,
		&code.Value,
		&code.Status,
		&code.MaxUses,
		&code.UsedCount,
		&usedBy,
		&usedAt,
		&code.CreatedAt,
		&expiresAt,
		&groupID,
		&code.ValidityDays,
		&notes,
		&groupRowID,
		&groupName,
		&groupPlatform,
		&groupRate,
		&groupSubscription,
	); err != nil {
		return code, err
	}
	code.UsedBy = &usedBy
	code.UsedAt = &usedAt
	code.Status = service.StatusUsed
	if expiresAt.Valid {
		code.ExpiresAt = &expiresAt.Time
	}
	if groupID.Valid {
		code.GroupID = &groupID.Int64
	}
	if notes.Valid {
		code.Notes = notes.String
	}
	if code.MaxUses <= 0 {
		code.MaxUses = 1
	}
	if groupRowID.Valid {
		code.Group = &service.Group{
			ID:               groupRowID.Int64,
			Name:             groupName.String,
			Platform:         groupPlatform.String,
			RateMultiplier:   groupRate.Float64,
			SubscriptionType: groupSubscription.String,
		}
	}
	return code, nil
}

func redeemCodeEntityToService(m *dbent.RedeemCode) *service.RedeemCode {
	if m == nil {
		return nil
	}
	out := &service.RedeemCode{
		ID:           m.ID,
		Code:         m.Code,
		Type:         m.Type,
		Value:        m.Value,
		Status:       m.Status,
		MaxUses:      1,
		UsedCount:    0,
		UsedBy:       m.UsedBy,
		UsedAt:       m.UsedAt,
		Notes:        derefString(m.Notes),
		CreatedAt:    m.CreatedAt,
		ExpiresAt:    m.ExpiresAt,
		GroupID:      m.GroupID,
		ValidityDays: m.ValidityDays,
	}
	if m.Edges.User != nil {
		out.User = userEntityToService(m.Edges.User)
	}
	if m.Edges.Group != nil {
		out.Group = groupEntityToService(m.Edges.Group)
	}
	return out
}

func normalizeRedeemMaxUses(maxUses int) int {
	if maxUses <= 0 {
		return 1
	}
	return maxUses
}

func (r *redeemCodeRepository) updateUsageLimits(ctx context.Context, id int64, maxUses, usedCount int) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.ExecContext(ctx, `
UPDATE redeem_codes
SET max_uses = $2, used_count = $3
WHERE id = $1
`, id, normalizeRedeemMaxUses(maxUses), usedCount)
	return err
}

func (r *redeemCodeRepository) fillUsageLimitsForValues(ctx context.Context, codes []service.RedeemCode) error {
	ptrs := make([]*service.RedeemCode, 0, len(codes))
	for i := range codes {
		ptrs = append(ptrs, &codes[i])
	}
	return r.fillUsageLimits(ctx, ptrs)
}

func (r *redeemCodeRepository) fillUsageLimits(ctx context.Context, codes []*service.RedeemCode) error {
	for _, code := range codes {
		if code == nil {
			continue
		}
		client := clientFromContext(ctx, r.client)
		rows, err := client.QueryContext(ctx, `
SELECT max_uses, used_count
FROM redeem_codes
WHERE id = $1
`, code.ID)
		if err != nil {
			if isUndefinedColumn(err) {
				return nil
			}
			return err
		}
		if rows.Next() {
			if err := rows.Scan(&code.MaxUses, &code.UsedCount); err != nil {
				_ = rows.Close()
				return err
			}
			code.MaxUses = normalizeRedeemMaxUses(code.MaxUses)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	return nil
}

func isUndefinedColumn(err error) bool {
	var pqErr *pq.Error
	return err != nil && errors.As(err, &pqErr) && pqErr.Code == "42703"
}

func isMissingRedeemUsageTable(err error) bool {
	if err == nil {
		return false
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "42P01" || pqErr.Code == "42703"
	}
	return strings.Contains(err.Error(), "redeem_code_usages")
}

func redeemCodeEntitiesToService(models []*dbent.RedeemCode) []service.RedeemCode {
	out := make([]service.RedeemCode, 0, len(models))
	for i := range models {
		if s := redeemCodeEntityToService(models[i]); s != nil {
			out = append(out, *s)
		}
	}
	return out
}
