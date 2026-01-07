Ent Adapter
====

[![Go Report Card](https://goreportcard.com/badge/github.com/casbin/ent-adapter)](https://goreportcard.com/report/github.com/casbin/ent-adapter)
[![Go](https://github.com/casbin/ent-adapter/actions/workflows/ci.yml/badge.svg)](https://github.com/casbin/ent-adapter/actions/workflows/ci.yml)
[![Coverage Status](https://coveralls.io/repos/github/casbin/ent-adapter/badge.svg?branch=master)](https://coveralls.io/github/casbin/ent-adapter?branch=master)
[![Godoc](https://godoc.org/github.com/casbin/ent-adapter?status.svg)](https://godoc.org/github.com/casbin/ent-adapter)
[![Release](https://img.shields.io/github/release/casbin/ent-adapter.svg)](https://github.com/casbin/ent-adapter/releases/latest)
[![Discord](https://img.shields.io/discord/1022748306096537660?logo=discord&label=discord&color=5865F2)](https://discord.gg/S5UjpzGZjN)
[![Sourcegraph](https://sourcegraph.com/github.com/casbin/ent-adapter/-/badge.svg)](https://sourcegraph.com/github.com/casbin/ent-adapter?badge)

Ent Adapter is the [Ent](https://entgo.io/) adapter for [Casbin](https://github.com/casbin/casbin). With this library, Casbin can load policy from Ent-supported databases or save policy to them.

Based on [Ent Supported Drivers](https://entgo.io/docs/sql-integration), the current supported databases are:

- MySQL
- PostgreSQL
- SQLite
- Gremlin

## Installation

```bash
go get github.com/casbin/ent-adapter
```

## Simple MySQL Example

```go
package main

import (
	"github.com/casbin/casbin/v3"
	entadapter "github.com/casbin/ent-adapter"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// Initialize an Ent adapter and use it in a Casbin enforcer:
	// The adapter will use the MySQL database named "casbin".
	// The database should be created manually before using the adapter.
	a, _ := entadapter.NewAdapter("mysql", "root:@tcp(127.0.0.1:3306)/casbin") // Your driver and data source.
	e, _ := casbin.NewEnforcer("examples/rbac_model.conf", a)
	
	// Load the policy from DB.
	e.LoadPolicy()
	
	// Check the permission.
	e.Enforce("alice", "data1", "read")
	
	// Modify the policy.
	// e.AddPolicy(...)
	// e.RemovePolicy(...)
	
	// Save the policy back to DB.
	e.SavePolicy()
}
```

## Simple PostgreSQL Example

```go
package main

import (
	"github.com/casbin/casbin/v3"
	entadapter "github.com/casbin/ent-adapter"
	_ "github.com/lib/pq"
)

func main() {
	// Initialize an Ent adapter and use it in a Casbin enforcer:
	// The adapter will use the PostgreSQL database named "casbin".
	// The database should be created manually before using the adapter.
	a, _ := entadapter.NewAdapter("postgres", "user=postgres password=postgres host=127.0.0.1 port=5432 sslmode=disable dbname=casbin") // Your driver and data source.
	e, _ := casbin.NewEnforcer("examples/rbac_model.conf", a)
	
	// Load the policy from DB.
	e.LoadPolicy()
	
	// Check the permission.
	e.Enforce("alice", "data1", "read")
	
	// Modify the policy.
	// e.AddPolicy(...)
	// e.RemovePolicy(...)
	
	// Save the policy back to DB.
	e.SavePolicy()
}
```

## Use NewAdapterWithClient

You can also create an adapter with an existing Ent client instance:

```go
package main

import (
	"github.com/casbin/casbin/v3"
	entadapter "github.com/casbin/ent-adapter"
	"github.com/casbin/ent-adapter/ent"
)

func main() {
	// Create an Ent client
	client, _ := ent.Open("mysql", "root:@tcp(127.0.0.1:3306)/casbin")
	
	// Initialize an Ent adapter with the client
	a, _ := entadapter.NewAdapterWithClient(client)
	e, _ := casbin.NewEnforcer("examples/rbac_model.conf", a)
	
	// Load the policy from DB.
	e.LoadPolicy()
	
	// Check the permission.
	e.Enforce("alice", "data1", "read")
	
	// Save the policy back to DB.
	e.SavePolicy()
}
```

## Database Configuration

The database used in the adapter should be created manually before calling `NewAdapter`. The adapter will automatically create the `casbin_rule` table if it doesn't exist.

## Getting Help

- [Casbin](https://github.com/casbin/casbin)

## License

This project is under Apache 2.0 License. See the [LICENSE](LICENSE) file for the full license text.
