package schema

import (
	"entgo.io/ent"
	"github.com/casbin/ent-adapter/template"
)

// CasbinRule holds the schema definition for the CasbinRule entity.
type CasbinRule struct {
	ent.Schema
}

func (CasbinRule) Mixin() []ent.Mixin {
	return []ent.Mixin{
		template.CasbinRuleMixin{},
	}
}
