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

// UpstreamGroupMultiplierHistory 仅在上游分组首次出现或倍率变化时保存一个点。
type UpstreamGroupMultiplierHistory struct {
	ent.Schema
}

func (UpstreamGroupMultiplierHistory) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "upstream_group_multiplier_history"}}
}

func (UpstreamGroupMultiplierHistory) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("site_id"),
		field.String("remote_id").NotEmpty().MaxLen(100),
		field.String("name").NotEmpty().MaxLen(100),
		field.String("platform").Default("").MaxLen(50),
		field.String("description").Default("").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Float("multiplier").Optional().Nillable(),
		field.Time("recorded_at").Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (UpstreamGroupMultiplierHistory) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("site", UpstreamSite.Type).Ref("group_multiplier_history").Field("site_id").Unique().Required(),
	}
}

func (UpstreamGroupMultiplierHistory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("site_id", "remote_id", "recorded_at"),
	}
}
