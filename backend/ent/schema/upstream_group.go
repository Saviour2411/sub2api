package schema

import (
	"time"

	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UpstreamGroup 保存上游当前可用分组及其当日指标。
type UpstreamGroup struct {
	ent.Schema
}

func (UpstreamGroup) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "upstream_groups"}}
}

func (UpstreamGroup) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (UpstreamGroup) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("site_id"),
		field.String("remote_id").NotEmpty().MaxLen(100),
		field.String("name").NotEmpty().MaxLen(100),
		field.String("platform").Default("").MaxLen(50),
		field.Float("multiplier").Optional().Nillable(),
		field.Int64("today_tokens").Default(0),
		field.Float("today_cost_usd").Default(0),
		field.Time("last_synced_at").Default(time.Now),
	}
}

func (UpstreamGroup) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("site", UpstreamSite.Type).Ref("groups").Field("site_id").Unique().Required(),
	}
}

func (UpstreamGroup) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("site_id", "remote_id").Unique(),
		index.Fields("site_id", "name"),
	}
}
