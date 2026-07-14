package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UpstreamDailyStat 保存站点按 Asia/Shanghai 自然日聚合的长期历史。
type UpstreamDailyStat struct {
	ent.Schema
}

func (UpstreamDailyStat) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "upstream_daily_stats"}}
}

func (UpstreamDailyStat) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("site_id"),
		field.Time("usage_date").SchemaType(map[string]string{dialect.Postgres: "date"}),
		field.Float("balance_usd").Optional().Nillable(),
		field.Int64("tokens").Default(0),
		field.Float("cost_usd").Default(0),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (UpstreamDailyStat) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("site", UpstreamSite.Type).Ref("daily_stats").Field("site_id").Unique().Required(),
	}
}

func (UpstreamDailyStat) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("site_id", "usage_date").Unique(),
		index.Fields("usage_date"),
	}
}
