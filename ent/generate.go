package ent

// The sql/upsert feature generates the OnConflict builders the adapter uses to
// make policy inserts idempotent against the unique index on (ptype, v0..v5).
//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate --feature sql/upsert ./schema
