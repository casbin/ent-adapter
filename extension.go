package entadapter

import (
	"fmt"
	"path/filepath"
	"runtime"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
)

type CasbinExtension struct {
	entc.DefaultExtension
}

func (*CasbinExtension) Templates() []*gen.Template {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		_ = fmt.Errorf("error retrieving current file path")
	}

	dir := filepath.Dir(filename)

	relativePath := filepath.Join(dir, "casbin.tmpl")

	return []*gen.Template{
		gen.MustParse(gen.NewTemplate("casbin").ParseFiles(relativePath)),
	}
}
