// Copyright 2021 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package entadapter

import (
	"log"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/util"
	"github.com/casbin/ent-adapter/examples/ent"
	"github.com/stretchr/testify/assert"
)

func testGetPolicy(t *testing.T, e *casbin.Enforcer, res [][]string) {
	myRes := e.GetPolicy()
	log.Print("Policy: ", myRes)

	if !util.Array2DEquals(res, myRes) {
		t.Error("Policy: ", myRes, ", supposed to be ", res)
	}
}

func testGetPolicyWithoutOrder(t *testing.T, e *casbin.Enforcer, res [][]string) {
	myRes := e.GetPolicy()
	log.Print("Policy: ", myRes)

	if !arrayEqualsWithoutOrder(myRes, res) {
		t.Error("Policy: ", myRes, ", supposed to be ", res)
	}
}

func arrayEqualsWithoutOrder(a [][]string, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}

	mapA := make(map[int]string)
	mapB := make(map[int]string)
	order := make(map[int]struct{})
	l := len(a)

	for i := 0; i < l; i++ {
		mapA[i] = util.ArrayToString(a[i])
		mapB[i] = util.ArrayToString(b[i])
	}

	for i := 0; i < l; i++ {
		for j := 0; j < l; j++ {
			if _, ok := order[j]; ok {
				if j == l-1 {
					return false
				} else {
					continue
				}
			}
			if mapA[i] == mapB[j] {
				order[j] = struct{}{}
				break
			} else if j == l-1 {
				return false
			}
		}
	}
	return true
}

func initPolicy(t *testing.T, a *ent.Adapter) {
	// Because the DB is empty at first,
	// so we need to load the policy from the file adapter (.CSV) first.
	e, err := casbin.NewEnforcer("examples/rbac_model.conf", "examples/rbac_policy.csv")
	if err != nil {
		panic(err)
	}

	// This is a trick to save the current policy to the DB.
	// We can't call e.SavePolicy() because the adapter in the enforcer is still the file adapter.
	// The current policy means the policy in the Casbin enforcer (aka in memory).
	err = a.SavePolicy(e.GetModel())
	if err != nil {
		panic(err)
	}

	// Clear the current policy.
	e.ClearPolicy()
	testGetPolicy(t, e, [][]string{})

	// Load the policy from DB.
	err = a.LoadPolicy(e.GetModel())
	if err != nil {
		panic(err)
	}
	testGetPolicy(t, e, [][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}})
}

func testSaveLoad(t *testing.T, a *ent.Adapter) {
	// Initialize some policy in DB.
	initPolicy(t, a)
	// Note: you don't need to look at the above code
	// if you already have a working DB with policy inside.

	// Now the DB has policy, so we can provide a normal use case.
	// Create an adapter and an enforcer.
	// NewEnforcer() will load the policy automatically.

	e, _ := casbin.NewEnforcer("examples/rbac_model.conf", a)
	testGetPolicy(t, e, [][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}})
}

func initAdapter(t *testing.T, driverName string, dataSourceName string) *ent.Adapter {
	// Create an adapter
	a, err := ent.NewAdapter(driverName, dataSourceName)
	if err != nil {
		panic(err)
	}

	// Initialize some policy in DB.
	initPolicy(t, a)
	// Now the DB has policy, so we can provide a normal use case.
	// Note: you don't need to look at the above code
	// if you already have a working DB with policy inside.

	return a
}

func initAdapterWithClientInstance(t *testing.T, client *ent.Client) *ent.Adapter {
	// Create an adapter
	a, _ := ent.NewAdapterWithClient(client)
	// Initialize some policy in DB.
	initPolicy(t, a)
	// Now the DB has policy, so we can provide a normal use case.
	// Note: you don't need to look at the above code
	// if you already have a working DB with policy inside.

	return a
}

func testAutoSave(t *testing.T, a *ent.Adapter) {

	// NewEnforcer() will load the policy automatically.
	e, _ := casbin.NewEnforcer("examples/rbac_model.conf", a)
	// AutoSave is enabled by default.
	// Now we disable it.
	e.EnableAutoSave(false)

	// Because AutoSave is disabled, the policy change only affects the policy in Casbin enforcer,
	// it doesn't affect the policy in the storage.
	e.AddPolicy("alice", "data1", "write")
	// Reload the policy from the storage to see the effect.
	e.LoadPolicy()
	// This is still the original policy.
	testGetPolicy(t, e, [][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}})

	// Now we enable the AutoSave.
	e.EnableAutoSave(true)

	// Because AutoSave is enabled, the policy change not only affects the policy in Casbin enforcer,
	// but also affects the policy in the storage.
	e.AddPolicy("alice", "data1", "write")
	// Reload the policy from the storage to see the effect.
	e.LoadPolicy()
	// The policy has a new rule: {"alice", "data1", "write"}.
	testGetPolicy(t, e, [][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}, {"alice", "data1", "write"}})

	// Remove the added rule.
	e.RemovePolicy("alice", "data1", "write")
	e.LoadPolicy()
	testGetPolicy(t, e, [][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}})

	// Remove "data2_admin" related policy rules via a filter.
	// Two rules: {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"} are deleted.
	e.RemoveFilteredPolicy(0, "data2_admin")
	e.LoadPolicy()
	testGetPolicy(t, e, [][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}})

	e.RemovePolicies([][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}})
	e.LoadPolicy()
	testGetPolicy(t, e, [][]string{})
}

//func testFilteredPolicy(t *testing.T, a *Adapter) {
//	// NewEnforcer() without an adapter will not auto load the policy
//	e, _ := casbin.NewEnforcer("examples/rbac_model.conf")
//	// Now set the adapter
//	e.SetAdapter(a)
//
//	// Load only alice's policies
//	assert.Nil(t, e.LoadFilteredPolicy(Filter{V0: []string{"alice"}}))
//	testGetPolicy(t, e, [][]string{{"alice", "data1", "read"}})
//
//	// Load only bob's policies
//	assert.Nil(t, e.LoadFilteredPolicy(Filter{V0: []string{"bob"}}))
//	testGetPolicy(t, e, [][]string{{"bob", "data2", "write"}})
//
//	// Load policies for data2_admin
//	assert.Nil(t, e.LoadFilteredPolicy(Filter{V0: []string{"data2_admin"}}))
//	testGetPolicy(t, e, [][]string{{"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}})
//
//	// Load policies for alice and bob
//	assert.Nil(t, e.LoadFilteredPolicy(Filter{V0: []string{"alice", "bob"}}))
//	testGetPolicy(t, e, [][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}})
//}

func testUpdatePolicy(t *testing.T, a *ent.Adapter) {
	// NewEnforcer() will load the policy automatically.
	e, _ := casbin.NewEnforcer("examples/rbac_model.conf", a)

	e.EnableAutoSave(true)
	e.UpdatePolicy([]string{"alice", "data1", "read"}, []string{"alice", "data1", "write"})
	e.LoadPolicy()
	testGetPolicy(t, e, [][]string{{"alice", "data1", "write"}, {"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}})
}

func testUpdatePolicies(t *testing.T, a *ent.Adapter) {
	// NewEnforcer() will load the policy automatically.
	e, _ := casbin.NewEnforcer("examples/rbac_model.conf", a)

	e.EnableAutoSave(true)
	e.UpdatePolicies([][]string{{"alice", "data1", "write"}, {"bob", "data2", "write"}}, [][]string{{"alice", "data1", "read"}, {"bob", "data2", "read"}})
	e.LoadPolicy()
	testGetPolicyWithoutOrder(t, e, [][]string{{"alice", "data1", "read"}, {"bob", "data2", "read"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}})
}

func testUpdateFilteredPolicies(t *testing.T, a *ent.Adapter) {
	// NewEnforcer() will load the policy automatically.
	e, _ := casbin.NewEnforcer("examples/rbac_model.conf", a)

	e.EnableAutoSave(true)
	e.UpdateFilteredPolicies([][]string{{"alice", "data1", "write"}}, 0, "alice", "data1", "read")
	e.UpdateFilteredPolicies([][]string{{"bob", "data2", "read"}}, 0, "bob", "data2")
	e.LoadPolicy()
	testGetPolicyWithoutOrder(t, e, [][]string{{"alice", "data1", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}, {"bob", "data2", "read"}})
}

func testFilteredPolicy(t *testing.T, a *ent.Adapter) {
	// NewEnforcer() without an adapter will not auto load the policy
	e, _ := casbin.NewEnforcer("examples/rbac_model.conf", "examples/rbac_policy.csv")

	// Now set the adapter
	e.SetAdapter(a)

	assert.Nil(t, e.SavePolicy())

	// Load only alice's policies
	assert.Nil(t, e.LoadFilteredPolicy(ent.Filter{V0: []string{"alice"}}))
	testGetPolicy(t, e, [][]string{{"alice", "data1", "read"}})

	// Load only bob's policies
	assert.Nil(t, e.LoadFilteredPolicy(ent.Filter{V0: []string{"bob"}}))
	testGetPolicy(t, e, [][]string{{"bob", "data2", "write"}})

	// Load policies for data2_admin
	assert.Nil(t, e.LoadFilteredPolicy(ent.Filter{V0: []string{"data2_admin"}}))
	testGetPolicy(t, e, [][]string{{"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}})

	// Load policies for alice and bob
	assert.Nil(t, e.LoadFilteredPolicy(ent.Filter{V0: []string{"alice", "bob"}}))
	testGetPolicy(t, e, [][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}})
}

func TestAdapters(t *testing.T) {
	a := initAdapter(t, "mysql", "root:@tcp(127.0.0.1:3306)/casbin")
	testAutoSave(t, a)
	testSaveLoad(t, a)

	a = initAdapter(t, "postgres", "user=postgres password=postgres host=127.0.0.1 port=5432 sslmode=disable dbname=casbin")
	testAutoSave(t, a)
	testSaveLoad(t, a)

	db, err := ent.Open("mysql", "root:@tcp(127.0.0.1:3306)/casbin")
	if err != nil {
		panic(err)
	}
	a = initAdapterWithClientInstance(t, db)
	testAutoSave(t, a)
	testSaveLoad(t, a)

	db, err = ent.Open("postgres", "user=postgres password=postgres host=127.0.0.1 port=5432 sslmode=disable dbname=casbin")
	if err != nil {
		panic(err)
	}
	a = initAdapterWithClientInstance(t, db)
	testAutoSave(t, a)
	testSaveLoad(t, a)

	a = initAdapter(t, "mysql", "root:@tcp(127.0.0.1:3306)/casbin")
	testUpdatePolicy(t, a)
	testUpdatePolicies(t, a)
	testUpdateFilteredPolicies(t, a)

	a = initAdapter(t, "postgres", "user=postgres password=postgres host=127.0.0.1 port=5432 sslmode=disable dbname=casbin")
	testUpdatePolicy(t, a)
	testUpdatePolicies(t, a)
	testUpdateFilteredPolicies(t, a)
}
