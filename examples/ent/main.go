//go:build ignore
// +build ignore

package main

import (
	"log"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"github.com/casbin/ent-adapter"
)

func main() {
	err := entc.Generate("./schema",
		&gen.Config{},
		entc.Extensions(&entadapter.CasbinExtension{}),
	)
	if err != nil {
		log.Fatal("running ent codegen:", err)
	}
}
