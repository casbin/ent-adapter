package template

import (
	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
)

type CasbinExtension struct {
	entc.DefaultExtension
}

func (*CasbinExtension) Templates() []*gen.Template {
	return []*gen.Template{
		gen.MustParse(gen.NewTemplate("casbin").ParseFiles("../template/casbin.tmpl")),
	}
}
