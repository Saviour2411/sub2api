package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"strings"
	"time"
)

type blueGreenMigration struct {
	Filename string `json:"filename"`
	Checksum string `json:"checksum"`
	Table    string `json:"table,omitempty"`
	Column   string `json:"column,omitempty"`
	DataType string `json:"data_type,omitempty"`
	Profile  string `json:"profile,omitempty"`
}

type blueGreenMigrationPolicy struct {
	Version    int                  `json:"version"`
	Migrations []blueGreenMigration `json:"migrations"`
}

var blueGreenIdentifier = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)
var blueGreenFilename = regexp.MustCompile(`^[0-9][a-zA-Z0-9_]+\.sql$`)
var blueGreenChecksum = regexp.MustCompile(`^[a-f0-9]{64}$`)
var blueGreenAddColumn = regexp.MustCompile(`(?i)^ALTER\s+TABLE\s+([a-z_][a-z0-9_]*)\s+ADD\s+COLUMN\s+IF\s+NOT\s+EXISTS\s+([a-z_][a-z0-9_]*)\s+(jsonb|text|boolean|smallint|integer|bigint|uuid)\s*;$`)

func parseBlueGreenMigrationPolicy(raw string) (*blueGreenMigrationPolicy, error) {
	if raw == "" {
		return nil, nil
	}
	var policy blueGreenMigrationPolicy
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return nil, fmt.Errorf("蓝绿迁移策略格式无效：%w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF || policy.Version != 1 || policy.Migrations == nil {
		return nil, fmt.Errorf("蓝绿迁移策略版本或清单无效")
	}
	seen := map[string]bool{}
	for _, entry := range policy.Migrations {
		if !blueGreenFilename.MatchString(entry.Filename) || !blueGreenChecksum.MatchString(entry.Checksum) || seen[entry.Filename] {
			return nil, fmt.Errorf("蓝绿迁移策略包含无效或重复条目")
		}
		seen[entry.Filename] = true
		if entry.Profile != "" {
			if !approvedBlueGreenSpecial(entry) {
				return nil, fmt.Errorf("蓝绿专项迁移未经批准")
			}
			continue
		}
		if !blueGreenIdentifier.MatchString(entry.Table) || strings.HasPrefix(entry.Table, "pg_") || !blueGreenIdentifier.MatchString(entry.Column) {
			return nil, fmt.Errorf("蓝绿迁移字段或表名无效")
		}
		switch entry.DataType {
		case "jsonb", "text", "boolean", "smallint", "integer", "bigint", "uuid":
		default:
			return nil, fmt.Errorf("蓝绿迁移字段类型未经批准")
		}
	}
	return &policy, nil
}

func (p *blueGreenMigrationPolicy) find(name string) (blueGreenMigration, bool) {
	for _, entry := range p.Migrations {
		if entry.Filename == name {
			return entry, true
		}
	}
	return blueGreenMigration{}, false
}

func validateBlueGreenSQL(entry blueGreenMigration, content string) error {
	sum := sha256.Sum256([]byte(strings.TrimSpace(content)))
	if hex.EncodeToString(sum[:]) != entry.Checksum {
		return fmt.Errorf("蓝绿迁移摘要不符：%s", entry.Filename)
	}
	if entry.Profile != "" {
		if !approvedBlueGreenSpecial(entry) {
			return fmt.Errorf("蓝绿专项迁移未经批准：%s", entry.Filename)
		}
		return nil
	}
	lines := []string{}
	for _, line := range strings.Split(content, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			lines = append(lines, line)
		}
	}
	match := blueGreenAddColumn.FindStringSubmatch(strings.TrimSpace(strings.Join(lines, "\n")))
	if len(match) != 4 || strings.ToLower(match[1]) != entry.Table || strings.ToLower(match[2]) != entry.Column || strings.ToLower(match[3]) != entry.DataType {
		return fmt.Errorf("蓝绿迁移不是获批的单条可空字段扩展：%s", entry.Filename)
	}
	return nil
}

func validateBlueGreenPendingMigrations(ctx context.Context, conn *sql.Conn, fsys fs.FS, policy *blueGreenMigrationPolicy) error {
	// 先一次性核对完整清单，不能先执行获批文件再发现其他待执行迁移。
	rows, err := conn.QueryContext(ctx, "SELECT filename, checksum FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("蓝绿迁移基线不存在或不可读：%w", err)
	}
	applied := map[string]string{}
	for rows.Next() {
		var name, checksum string
		if err := rows.Scan(&name, &checksum); err != nil {
			_ = rows.Close()
			return err
		}
		applied[name] = checksum
	}
	rowErr := rows.Err()
	_ = rows.Close()
	if rowErr != nil {
		return rowErr
	}
	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return err
	}
	for _, entry := range policy.Migrations {
		content, err := fs.ReadFile(fsys, entry.Filename)
		if err != nil {
			return fmt.Errorf("获批迁移文件不存在：%s", entry.Filename)
		}
		if err := validateBlueGreenSQL(entry, string(content)); err != nil {
			return err
		}
	}
	for _, name := range files {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		content := strings.TrimSpace(string(raw))
		if content == "" {
			continue
		}
		sum := sha256.Sum256([]byte(content))
		checksum := hex.EncodeToString(sum[:])
		if previous, ok := applied[name]; ok {
			if previous != checksum && !isMigrationChecksumCompatible(name, previous, checksum) {
				return fmt.Errorf("蓝绿历史迁移摘要不符：%s", name)
			}
		} else if _, ok := policy.find(name); !ok {
			return fmt.Errorf("蓝绿待执行迁移未获批准：%s", name)
		}
	}
	return nil
}

func applyBlueGreenExpansion(ctx context.Context, conn *sql.Conn, entry blueGreenMigration, content string) error {
	if err := validateBlueGreenSQL(entry, content); err != nil {
		return err
	}
	if entry.Profile != "" {
		return applyBlueGreenSpecial(ctx, conn, entry, content)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// NOWAIT 不在业务查询后排队；事务级设置不泄漏到业务连接池。
	for _, statement := range []string{
		"SET LOCAL lock_timeout = '1s'", "SET LOCAL statement_timeout = '5s'",
		"SET LOCAL search_path = pg_catalog, public",
		fmt.Sprintf(`LOCK TABLE ONLY public."%s" IN ACCESS EXCLUSIVE MODE NOWAIT`, entry.Table),
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("蓝绿迁移保护未通过，保留旧实例：%w", err)
		}
	}
	var ordinary bool
	if err := tx.QueryRowContext(ctx, `SELECT c.relkind = 'r' AND c.reloftype = 0 AND NOT EXISTS
        (SELECT 1 FROM pg_inherits i WHERE i.inhparent = c.oid OR i.inhrelid = c.oid)
        FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public' AND c.relname = $1`, entry.Table).Scan(&ordinary); err != nil || !ordinary {
		return fmt.Errorf("蓝绿迁移仅允许既有普通表，拒绝分区、继承或类型表")
	}
	if _, err := tx.ExecContext(ctx, content); err != nil {
		return fmt.Errorf("执行蓝绿字段扩展失败：%w", err)
	}
	var compatible bool
	if err := tx.QueryRowContext(ctx, `SELECT a.atttypid = to_regtype($3) AND NOT a.attnotnull
        AND NOT a.atthasdef AND a.attidentity = '' AND a.attgenerated = ''
        AND NOT EXISTS (SELECT 1 FROM pg_constraint k WHERE k.conrelid = c.oid
            AND (k.conkey IS NULL OR a.attnum = ANY(k.conkey)))
        FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public' AND c.relname = $1 AND a.attname = $2 AND NOT a.attisdropped`,
		entry.Table, entry.Column, entry.DataType).Scan(&compatible); err != nil || !compatible {
		return fmt.Errorf("蓝绿新增字段实际类型、可空性或默认值不符")
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (filename, checksum) VALUES ($1, $2)", entry.Filename, entry.Checksum); err != nil {
		return err
	}
	return tx.Commit()
}
