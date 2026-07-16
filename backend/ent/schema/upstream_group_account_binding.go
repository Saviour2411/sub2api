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

// UpstreamGroupAccountBinding 保存上游分组与本地分组账号的绑定关系。
type UpstreamGroupAccountBinding struct {
	ent.Schema
}

func (UpstreamGroupAccountBinding) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "upstream_group_account_bindings"}}
}

func (UpstreamGroupAccountBinding) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("upstream_group_id"),
		field.Int64("local_group_id"),
		field.Int64("account_id").Unique(),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (UpstreamGroupAccountBinding) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("upstream_group", UpstreamGroup.Type).
			Ref("account_bindings").
			Field("upstream_group_id").
			Unique().
			Required(),
		edge.From("local_group", Group.Type).
			Ref("upstream_group_account_bindings").
			Field("local_group_id").
			Unique().
			Required(),
		edge.From("account", Account.Type).
			Ref("upstream_group_account_bindings").
			Field("account_id").
			Unique().
			Required(),
	}
}

func (UpstreamGroupAccountBinding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("upstream_group_id"),
		index.Fields("local_group_id"),
	}
}
