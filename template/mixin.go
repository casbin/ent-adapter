package template

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
)

type CasbinRuleMixin struct {
	mixin.Schema
}

func (CasbinRuleMixin) Fields() []ent.Field {
	return []ent.Field{
		field.String("Ptype").Default(""),
		field.String("V0").Default(""),
		field.String("V1").Default(""),
		field.String("V2").Default(""),
		field.String("V3").Default(""),
		field.String("V4").Default(""),
		field.String("V5").Default(""),
	}
}

func (CasbinRuleMixin) Index() []ent.Index {
	return []ent.Index{
		index.Fields("Ptype", "V0", "V1", "V2", "V3", "V4", "V5").Unique(),
	}
}
