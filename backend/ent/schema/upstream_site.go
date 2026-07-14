package schema

import (
	"time"

	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UpstreamSite 保存二开功能中的独立上游站点配置与最新同步指标。
type UpstreamSite struct {
	ent.Schema
}

func (UpstreamSite) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "upstream_sites"}}
}

func (UpstreamSite) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (UpstreamSite) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty().MaxLen(100),
		field.String("base_url").NotEmpty().MaxLen(500),
		field.Enum("platform").Values("sub2api", "newapi"),
		field.Enum("auth_mode").Values("password", "token"),
		field.String("account").Default("").MaxLen(255),
		field.String("credential_encrypted").NotEmpty().Sensitive(),
		field.Bool("enabled").Default(true),
		field.Enum("status").Values("pending", "syncing", "healthy", "error").Default("pending"),
		field.String("error_message").Optional().Nillable().MaxLen(500),
		field.Float("balance_usd").Optional().Nillable(),
		field.Int64("today_tokens").Default(0),
		field.Float("today_cost_usd").Default(0),
		field.Int64("total_tokens").Default(0),
		field.Float("total_cost_usd").Default(0),
		field.Time("tracking_started_at").Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("last_synced_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("next_sync_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int64("created_by"),
	}
}

func (UpstreamSite) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("groups", UpstreamGroup.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("daily_stats", UpstreamDailyStat.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (UpstreamSite) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled", "next_sync_at"),
		index.Fields("platform"),
		index.Fields("status"),
		index.Fields("created_at"),
	}
}
