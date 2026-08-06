Ent Adapter
====

[![Lint](https://github.com/apache/casbin-ent-adapter/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/apache/casbin-ent-adapter/actions/workflows/golangci-lint.yml)
[![Build](https://github.com/apache/casbin-ent-adapter/actions/workflows/ci.yml/badge.svg)](https://github.com/apache/casbin-ent-adapter/actions/workflows/ci.yml)
[![Coverage Status](https://coveralls.io/repos/github/apache/casbin-ent-adapter/badge.svg?branch=master)](https://coveralls.io/github/apache/casbin-ent-adapter?branch=master)
[![Godoc](https://godoc.org/github.com/apache/casbin-ent-adapter?status.svg)](https://pkg.go.dev/github.com/apache/casbin-ent-adapter)
[![Release](https://img.shields.io/github/release/apache/casbin-ent-adapter.svg)](https://github.com/apache/casbin-ent-adapter/releases/latest)
[![Discord](https://img.shields.io/discord/1022748306096537660?logo=discord&label=discord&color=5865F2)](https://discord.gg/S5UjpzGZjN)

Ent Adapter is the [Ent](https://entgo.io/) adapter for [Casbin](https://github.com/apache/casbin). With this library, Casbin can load policy from Ent-supported databases or save policy to them.

Based on [Ent Supported Drivers](https://entgo.io/docs/sql-integration), the current supported databases are:

- MySQL
- PostgreSQL
- SQLite
- Gremlin

## Installation

```bash
go get github.com/apache/casbin-ent-adapter
```

## Simple MySQL Example

```go
package main

import (
	"github.com/casbin/casbin/v3"
	entadapter "github.com/apache/casbin-ent-adapter"
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
	entadapter "github.com/apache/casbin-ent-adapter"
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
	entadapter "github.com/apache/casbin-ent-adapter"
	"github.com/apache/casbin-ent-adapter/ent"
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

## Upgrading an existing database

A policy rule is now unique on `(ptype, v0, v1, v2, v3, v4, v5)`, and the seven
columns are `varchar(100)` so that the index fits within the 3072-byte key limit
InnoDB enforces. Both are enforced by the automatic migration that runs inside
`NewAdapter` and `NewAdapterWithClient`, so a database written by an earlier
version needs a look before the first start on this version. A fresh database
needs nothing.

Check for rules that no longer fit. The adapter runs this check itself and
refuses to migrate while it returns anything, because a MySQL server running
without strict mode would truncate the values rather than reject them:

```sql
SELECT * FROM casbin_rules
WHERE CHAR_LENGTH(ptype) > 100 OR CHAR_LENGTH(v0) > 100 OR CHAR_LENGTH(v1) > 100
   OR CHAR_LENGTH(v2) > 100 OR CHAR_LENGTH(v3) > 100 OR CHAR_LENGTH(v4) > 100
   OR CHAR_LENGTH(v5) > 100;
```

Check for rules stored more than once. Nothing enforced uniqueness before, so
duplicates may have accumulated, and the index cannot be built while they exist:

```sql
SELECT ptype, v0, v1, v2, v3, v4, v5, COUNT(*) AS copies
FROM casbin_rules GROUP BY ptype, v0, v1, v2, v3, v4, v5 HAVING COUNT(*) > 1;
```

Duplicates are redundant by definition — casbin evaluates a rule the same way
whether it is stored once or ten times — so keeping the lowest `id` of each
group is safe:

```sql
DELETE c FROM casbin_rules c
JOIN (
  SELECT MIN(id) AS keep_id, ptype, v0, v1, v2, v3, v4, v5
  FROM casbin_rules GROUP BY ptype, v0, v1, v2, v3, v4, v5 HAVING COUNT(*) > 1
) d ON c.ptype = d.ptype AND c.v0 = d.v0 AND c.v1 = d.v1 AND c.v2 = d.v2
   AND c.v3 = d.v3 AND c.v4 = d.v4 AND c.v5 = d.v5
WHERE c.id > d.keep_id;
```

On PostgreSQL, use `DELETE FROM casbin_rules WHERE id NOT IN (SELECT MIN(id)
FROM casbin_rules GROUP BY ptype, v0, v1, v2, v3, v4, v5);` instead.

Adding a rule that is already stored stays a successful no-op rather than
becoming an error, so replicas whose in-memory model has fallen behind another
writer keep working.

## Getting Help

- [Casbin](https://github.com/apache/casbin)

## License

This project is under Apache 2.0 License. See the [LICENSE](LICENSE) file for the full license text.
