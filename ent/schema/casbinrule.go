package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// MaxFieldLen is the maximum length of every policy column.
//
// The unique index below spans all seven policy columns. MySQL caps a single
// InnoDB index key at 3072 bytes, and ent creates its tables as utf8mb4, where
// a character costs up to 4 bytes. Leaving the columns at ent's default
// varchar(255) would need 7*255*4 = 7140 bytes and the index would be rejected
// with "Error 1071: Specified key was too long". At 100 the key needs
// 7*100*4 = 2800 bytes, which fits, and it matches the varchar(100) used by
// casbin's canonical SQL schema and by gorm-adapter.
const MaxFieldLen = 100

// CasbinRule holds the schema definition for the CasbinRule entity.
type CasbinRule struct {
	ent.Schema
}

// Fields of the CasbinRule.
//
// Every column is NOT NULL with an empty-string default. That matters for the
// unique index: SQL treats NULLs as distinct from each other, so nullable
// columns would silently defeat the constraint for every rule that leaves some
// of V0..V5 unused.
func (CasbinRule) Fields() []ent.Field {
	return []ent.Field{
		field.String("Ptype").MaxLen(MaxFieldLen).Default(""),
		field.String("V0").MaxLen(MaxFieldLen).Default(""),
		field.String("V1").MaxLen(MaxFieldLen).Default(""),
		field.String("V2").MaxLen(MaxFieldLen).Default(""),
		field.String("V3").MaxLen(MaxFieldLen).Default(""),
		field.String("V4").MaxLen(MaxFieldLen).Default(""),
		field.String("V5").MaxLen(MaxFieldLen).Default(""),
	}
}

// Edges of the CasbinRule.
func (CasbinRule) Edges() []ent.Edge {
	return nil
}

// Indexes of the CasbinRule.
//
// The method must be named Indexes: ent discovers schema indexes through this
// exact interface method, so a differently named method compiles fine and is
// silently ignored by codegen.
func (CasbinRule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("Ptype", "V0", "V1", "V2", "V3", "V4", "V5").Unique(),
	}
}
