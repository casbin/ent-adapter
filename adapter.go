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
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/casbin/casbin/v3/persist"
	"github.com/apache/casbin-ent-adapter/ent/casbinrule"
	"github.com/apache/casbin-ent-adapter/ent/predicate"

	"github.com/casbin/casbin/v3/model"
	"github.com/apache/casbin-ent-adapter/ent"
	"github.com/apache/casbin-ent-adapter/ent/schema"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"
	"github.com/pkg/errors"
)

const (
	DefaultTableName = "casbin_rule"
	DefaultDatabase  = "casbin"
)

// batchSize caps how many rows go into a single INSERT.
const batchSize = 5000

// policyColumns are the columns covered by the unique index on a policy rule.
// They double as the conflict target of every insert.
var policyColumns = []string{
	casbinrule.FieldPtype,
	casbinrule.FieldV0,
	casbinrule.FieldV1,
	casbinrule.FieldV2,
	casbinrule.FieldV3,
	casbinrule.FieldV4,
	casbinrule.FieldV5,
}

type Adapter struct {
	client *ent.Client
	ctx    context.Context

	filtered bool
}

type Filter struct {
	Ptype []string
	V0    []string
	V1    []string
	V2    []string
	V3    []string
	V4    []string
	V5    []string
}

type Option func(a *Adapter) error

func open(driverName, dataSourceName string) (*ent.Client, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	var drv dialect.Driver
	if driverName == "pgx" {
		drv = entsql.OpenDB(dialect.Postgres, db)
	} else {
		drv = entsql.OpenDB(driverName, db)
	}
	return ent.NewClient(ent.Driver(drv)), nil
}

// NewAdapter returns an adapter by driver name and data source string.
func NewAdapter(driverName, dataSourceName string, options ...Option) (*Adapter, error) {
	client, err := open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	a := &Adapter{
		client: client,
		ctx:    context.Background(),
	}
	for _, option := range options {
		if err := option(a); err != nil {
			return nil, err
		}
	}
	if err := a.migrate(); err != nil {
		return nil, err
	}
	return a, nil
}

// NewAdapterWithClient create an adapter with client passed in.
// This method does not ensure the existence of database, user should create database manually.
func NewAdapterWithClient(client *ent.Client, options ...Option) (*Adapter, error) {
	a := &Adapter{
		client: client,
		ctx:    context.Background(),
	}
	for _, option := range options {
		if err := option(a); err != nil {
			return nil, err
		}
	}
	if err := a.migrate(); err != nil {
		return nil, err
	}
	return a, nil
}

// migrate brings the schema up to date, guarding the two ways that can go wrong
// on a database created before the policy tuple became unique.
func (a *Adapter) migrate() error {
	if err := a.checkFieldLengths(); err != nil {
		return err
	}
	if err := a.client.Schema.Create(a.ctx); err != nil {
		return explainDuplicateRules(err)
	}
	return nil
}

// checkFieldLengths refuses to migrate a table still holding policy values
// longer than the column width that the unique index requires. Narrowing such a
// column fails outright on a MySQL server in strict mode, but a server without
// it truncates the values instead, quietly rewriting the stored policies.
//
// The probe is best effort. On a fresh database the table does not exist yet and
// the query fails, which says nothing about anyone's data, so probe errors are
// dropped and the migration itself is left to report real problems.
func (a *Adapter) checkFieldLengths() error {
	tooLong, err := a.client.CasbinRule.Query().Where(func(s *entsql.Selector) {
		// LENGTH counts bytes on MySQL, so the portable spelling there is
		// CHAR_LENGTH, which SQLite in turn does not have.
		length := "CHAR_LENGTH"
		if s.Dialect() == dialect.SQLite {
			length = "LENGTH"
		}
		overflows := make([]*entsql.Predicate, 0, len(policyColumns))
		for _, column := range policyColumns {
			overflows = append(overflows, entsql.ExprP(
				fmt.Sprintf("%s(%s) > %d", length, s.C(column), schema.MaxFieldLen)))
		}
		s.Where(entsql.Or(overflows...))
	}).Exist(a.ctx)
	if err != nil || !tooLong {
		return nil
	}
	return fmt.Errorf("casbin_rules holds policy values longer than %d characters, "+
		"which the columns can no longer store; shorten them before upgrading, "+
		"otherwise the migration truncates them", schema.MaxFieldLen)
}

// explainDuplicateRules names the policy data behind a migration that failed
// because the table already holds rows the new unique index forbids. Drivers
// report it as a bare constraint violation naming only an index.
func explainDuplicateRules(err error) error {
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{"duplicate entry", "duplicate key value", "unique constraint failed"} {
		if strings.Contains(msg, marker) {
			return errors.Wrap(err, "casbin_rules holds duplicate policy rules, which the unique "+
				"index on (ptype, v0..v5) forbids; delete the redundant rows and retry")
		}
	}
	return err
}

// LoadPolicy loads all policy rules from the storage.
func (a *Adapter) LoadPolicy(model model.Model) error {
	policies, err := a.client.CasbinRule.Query().Order(ent.Asc("id")).All(a.ctx)
	if err != nil {
		return err
	}
	for _, policy := range policies {
		if err := loadPolicyLine(policy, model); err != nil {
			return err
		}
	}
	return nil
}

// LoadFilteredPolicy loads only policy rules that match the filter.
// Filter parameter here is a Filter structure
func (a *Adapter) LoadFilteredPolicy(model model.Model, filter interface{}) error {

	filterValue, ok := filter.(Filter)
	if !ok {
		return fmt.Errorf("invalid filter type: %v", reflect.TypeOf(filter))
	}

	session := a.client.CasbinRule.Query().Order(ent.Asc("id"))
	if len(filterValue.Ptype) != 0 {
		session.Where(casbinrule.PtypeIn(filterValue.Ptype...))
	}
	if len(filterValue.V0) != 0 {
		session.Where(casbinrule.V0In(filterValue.V0...))
	}
	if len(filterValue.V1) != 0 {
		session.Where(casbinrule.V1In(filterValue.V1...))
	}
	if len(filterValue.V2) != 0 {
		session.Where(casbinrule.V2In(filterValue.V2...))
	}
	if len(filterValue.V3) != 0 {
		session.Where(casbinrule.V3In(filterValue.V3...))
	}
	if len(filterValue.V4) != 0 {
		session.Where(casbinrule.V4In(filterValue.V4...))
	}
	if len(filterValue.V5) != 0 {
		session.Where(casbinrule.V5In(filterValue.V5...))
	}

	lines, err := session.All(a.ctx)
	if err != nil {
		return err
	}

	for _, line := range lines {
		if err := loadPolicyLine(line, model); err != nil {
			return err
		}
	}
	a.filtered = true

	return nil
}

// IsFiltered returns true if the loaded policy has been filtered.
func (a *Adapter) IsFiltered() bool {
	return a.filtered
}

// SavePolicy saves all policy rules to the storage.
func (a *Adapter) SavePolicy(model model.Model) error {
	return a.WithTx(func(tx *ent.Tx) error {
		if _, err := tx.CasbinRule.Delete().Exec(a.ctx); err != nil {
			return err
		}
		lines := make([]policyLine, 0)

		for _, sec := range []string{"p", "g"} {
			for ptype, ast := range model[sec] {
				lines = append(lines, policyLines(ptype, ast.Policy)...)
			}
		}

		return a.insertPolicyLines(tx, lines)
	})
}

// AddPolicy adds a policy rule to the storage.
// This is part of the Auto-Save feature.
func (a *Adapter) AddPolicy(sec string, ptype string, rule []string) error {
	return a.WithTx(func(tx *ent.Tx) error {
		return a.insertPolicyLines(tx, []policyLine{{ptype: ptype, rule: rule}})
	})
}

// RemovePolicy removes a policy rule from the storage.
// This is part of the Auto-Save feature.
func (a *Adapter) RemovePolicy(sec string, ptype string, rule []string) error {
	return a.WithTx(func(tx *ent.Tx) error {
		instance := a.toInstance(ptype, rule)
		_, err := tx.CasbinRule.Delete().Where(
			casbinrule.PtypeEQ(instance.Ptype),
			casbinrule.V0EQ(instance.V0),
			casbinrule.V1EQ(instance.V1),
			casbinrule.V2EQ(instance.V2),
			casbinrule.V3EQ(instance.V3),
			casbinrule.V4EQ(instance.V4),
			casbinrule.V5EQ(instance.V5),
		).Exec(a.ctx)
		return err
	})
}

// RemoveFilteredPolicy removes policy rules that match the filter from the storage.
// This is part of the Auto-Save feature.
func (a *Adapter) RemoveFilteredPolicy(sec string, ptype string, fieldIndex int, fieldValues ...string) error {
	return a.WithTx(func(tx *ent.Tx) error {
		cond := make([]predicate.CasbinRule, 0)
		cond = append(cond, casbinrule.PtypeEQ(ptype))
		if fieldIndex <= 0 && 0 < fieldIndex+len(fieldValues) && len(fieldValues[0-fieldIndex]) > 0 {
			cond = append(cond, casbinrule.V0EQ(fieldValues[0-fieldIndex]))
		}
		if fieldIndex <= 1 && 1 < fieldIndex+len(fieldValues) && len(fieldValues[1-fieldIndex]) > 0 {
			cond = append(cond, casbinrule.V1EQ(fieldValues[1-fieldIndex]))
		}
		if fieldIndex <= 2 && 2 < fieldIndex+len(fieldValues) && len(fieldValues[2-fieldIndex]) > 0 {
			cond = append(cond, casbinrule.V2EQ(fieldValues[2-fieldIndex]))
		}
		if fieldIndex <= 3 && 3 < fieldIndex+len(fieldValues) && len(fieldValues[3-fieldIndex]) > 0 {
			cond = append(cond, casbinrule.V3EQ(fieldValues[3-fieldIndex]))
		}
		if fieldIndex <= 4 && 4 < fieldIndex+len(fieldValues) && len(fieldValues[4-fieldIndex]) > 0 {
			cond = append(cond, casbinrule.V4EQ(fieldValues[4-fieldIndex]))
		}
		if fieldIndex <= 5 && 5 < fieldIndex+len(fieldValues) && len(fieldValues[5-fieldIndex]) > 0 {
			cond = append(cond, casbinrule.V5EQ(fieldValues[5-fieldIndex]))
		}
		_, err := tx.CasbinRule.Delete().Where(
			cond...,
		).Exec(a.ctx)
		return err
	})
}

// AddPolicies adds policy rules to the storage.
// This is part of the Auto-Save feature.
func (a *Adapter) AddPolicies(sec string, ptype string, rules [][]string) error {
	return a.WithTx(func(tx *ent.Tx) error {
		return a.createPolicies(tx, ptype, rules)
	})
}

// RemovePolicies removes policy rules from the storage.
// This is part of the Auto-Save feature.
func (a *Adapter) RemovePolicies(sec string, ptype string, rules [][]string) error {
	return a.WithTx(func(tx *ent.Tx) error {
		for _, rule := range rules {
			instance := a.toInstance(ptype, rule)
			if _, err := tx.CasbinRule.Delete().Where(
				casbinrule.PtypeEQ(instance.Ptype),
				casbinrule.V0EQ(instance.V0),
				casbinrule.V1EQ(instance.V1),
				casbinrule.V2EQ(instance.V2),
				casbinrule.V3EQ(instance.V3),
				casbinrule.V4EQ(instance.V4),
				casbinrule.V5EQ(instance.V5),
			).Exec(a.ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

func (a *Adapter) WithTx(fn func(tx *ent.Tx) error) error {
	tx, err := a.client.Tx(a.ctx)
	if err != nil {
		return err
	}
	defer func() {
		if v := recover(); v != nil {
			_ = tx.Rollback()
			panic(v)
		}
	}()
	if err := fn(tx); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			err = errors.Wrapf(err, "rolling back transaction: %v", rerr)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return errors.Wrapf(err, "committing transaction: %v", err)
	}
	return nil
}

func loadPolicyLine(line *ent.CasbinRule, model model.Model) error {
	var p = []string{line.Ptype,
		line.V0, line.V1, line.V2, line.V3, line.V4, line.V5}

	var lineText string
	if line.V5 != "" {
		lineText = strings.Join(p, ", ")
	} else if line.V4 != "" {
		lineText = strings.Join(p[:6], ", ")
	} else if line.V3 != "" {
		lineText = strings.Join(p[:5], ", ")
	} else if line.V2 != "" {
		lineText = strings.Join(p[:4], ", ")
	} else if line.V1 != "" {
		lineText = strings.Join(p[:3], ", ")
	} else if line.V0 != "" {
		lineText = strings.Join(p[:2], ", ")
	}

	return persist.LoadPolicyLine(lineText, model)
}

func (a *Adapter) toInstance(ptype string, rule []string) *ent.CasbinRule {
	instance := &ent.CasbinRule{}

	instance.Ptype = ptype

	if len(rule) > 0 {
		instance.V0 = rule[0]
	}
	if len(rule) > 1 {
		instance.V1 = rule[1]
	}
	if len(rule) > 2 {
		instance.V2 = rule[2]
	}
	if len(rule) > 3 {
		instance.V3 = rule[3]
	}
	if len(rule) > 4 {
		instance.V4 = rule[4]
	}
	if len(rule) > 5 {
		instance.V5 = rule[5]
	}
	return instance
}

func (a *Adapter) savePolicyLine(tx *ent.Tx, ptype string, rule []string) *ent.CasbinRuleCreate {
	line := tx.CasbinRule.Create()

	line.SetPtype(ptype)
	if len(rule) > 0 {
		line.SetV0(rule[0])
	}
	if len(rule) > 1 {
		line.SetV1(rule[1])
	}
	if len(rule) > 2 {
		line.SetV2(rule[2])
	}
	if len(rule) > 3 {
		line.SetV3(rule[3])
	}
	if len(rule) > 4 {
		line.SetV4(rule[4])
	}
	if len(rule) > 5 {
		line.SetV5(rule[5])
	}

	return line
}

// policyKey is the storage identity of a rule: the tuple covered by the unique
// index. Two rules sharing a key map to the same row.
type policyKey [7]string

func newPolicyKey(ptype string, rule []string) policyKey {
	key := policyKey{ptype}
	for i := 0; i < len(rule) && i+1 < len(key); i++ {
		key[i+1] = rule[i]
	}
	return key
}

// policyLine pairs a rule with its ptype so rules coming from different model
// sections can be deduplicated against each other.
type policyLine struct {
	ptype string
	rule  []string
}

func policyLines(ptype string, rules [][]string) []policyLine {
	lines := make([]policyLine, 0, len(rules))
	for _, rule := range rules {
		lines = append(lines, policyLine{ptype: ptype, rule: rule})
	}
	return lines
}

// dedupPolicyLines keeps the first line of every distinct key. A casbin model
// can legitimately carry duplicates -- loading a policy file appends each line
// without checking whether the model already holds it -- and writing them out
// unchanged would now collide with the unique index.
func dedupPolicyLines(lines []policyLine) []policyLine {
	seen := make(map[policyKey]struct{}, len(lines))
	out := make([]policyLine, 0, len(lines))
	for _, line := range lines {
		key := newPolicyKey(line.ptype, line.rule)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, line)
	}
	return out
}

// insertPolicyLines writes lines in batches, leaving rows that already exist
// untouched.
//
// Ignoring conflicts is what keeps writes idempotent now that the tuple is
// unique. The enforcer screens duplicates against its in-memory model, but that
// model is per-process: a second replica, or one whose model has gone stale,
// can still submit a rule that is already stored. That used to insert a second
// identical row; it must not start failing the caller instead.
func (a *Adapter) insertPolicyLines(tx *ent.Tx, lines []policyLine) error {
	lines = dedupPolicyLines(lines)
	for start := 0; start < len(lines); start += batchSize {
		end := start + batchSize
		if end > len(lines) {
			end = len(lines)
		}
		batch := make([]*ent.CasbinRuleCreate, 0, end-start)
		for _, line := range lines[start:end] {
			batch = append(batch, a.savePolicyLine(tx, line.ptype, line.rule))
		}
		// The conflict target is named explicitly because PostgreSQL requires
		// an inference specification for DO UPDATE. MySQL has no target in its
		// ON DUPLICATE KEY syntax and ignores it.
		if err := tx.CasbinRule.CreateBulk(batch...).
			OnConflictColumns(policyColumns...).
			Ignore().
			Exec(a.ctx); err != nil {
			return err
		}
	}
	return nil
}

// UpdatePolicy updates a policy rule from storage.
// This is part of the Auto-Save feature.
func (a *Adapter) UpdatePolicy(sec string, ptype string, oldRule, newPolicy []string) error {
	return a.WithTx(func(tx *ent.Tx) error {
		rule := a.toInstance(ptype, oldRule)
		line := tx.CasbinRule.Update().Where(
			casbinrule.PtypeEQ(rule.Ptype),
			casbinrule.V0EQ(rule.V0),
			casbinrule.V1EQ(rule.V1),
			casbinrule.V2EQ(rule.V2),
			casbinrule.V3EQ(rule.V3),
			casbinrule.V4EQ(rule.V4),
			casbinrule.V5EQ(rule.V5),
		)
		rule = a.toInstance(ptype, newPolicy)
		line.SetV0(rule.V0)
		line.SetV1(rule.V1)
		line.SetV2(rule.V2)
		line.SetV3(rule.V3)
		line.SetV4(rule.V4)
		line.SetV5(rule.V5)
		_, err := line.Save(a.ctx)
		return err
	})
}

// UpdatePolicies updates some policy rules to storage, like db, redis.
func (a *Adapter) UpdatePolicies(sec string, ptype string, oldRules, newRules [][]string) error {
	return a.WithTx(func(tx *ent.Tx) error {
		for _, policy := range oldRules {
			rule := a.toInstance(ptype, policy)
			if _, err := tx.CasbinRule.Delete().Where(
				casbinrule.PtypeEQ(rule.Ptype),
				casbinrule.V0EQ(rule.V0),
				casbinrule.V1EQ(rule.V1),
				casbinrule.V2EQ(rule.V2),
				casbinrule.V3EQ(rule.V3),
				casbinrule.V4EQ(rule.V4),
				casbinrule.V5EQ(rule.V5),
			).Exec(a.ctx); err != nil {
				return err
			}
		}
		return a.insertPolicyLines(tx, policyLines(ptype, newRules))
	})
}

// UpdateFilteredPolicies deletes old rules and adds new rules.
func (a *Adapter) UpdateFilteredPolicies(sec string, ptype string, newPolicies [][]string, fieldIndex int, fieldValues ...string) ([][]string, error) {
	oldPolicies := make([][]string, 0)
	err := a.WithTx(func(tx *ent.Tx) error {
		cond := make([]predicate.CasbinRule, 0)
		cond = append(cond, casbinrule.PtypeEQ(ptype))
		if fieldIndex <= 0 && 0 < fieldIndex+len(fieldValues) && len(fieldValues[0-fieldIndex]) > 0 {
			cond = append(cond, casbinrule.V0EQ(fieldValues[0-fieldIndex]))
		}
		if fieldIndex <= 1 && 1 < fieldIndex+len(fieldValues) && len(fieldValues[1-fieldIndex]) > 0 {
			cond = append(cond, casbinrule.V1EQ(fieldValues[1-fieldIndex]))
		}
		if fieldIndex <= 2 && 2 < fieldIndex+len(fieldValues) && len(fieldValues[2-fieldIndex]) > 0 {
			cond = append(cond, casbinrule.V2EQ(fieldValues[2-fieldIndex]))
		}
		if fieldIndex <= 3 && 3 < fieldIndex+len(fieldValues) && len(fieldValues[3-fieldIndex]) > 0 {
			cond = append(cond, casbinrule.V3EQ(fieldValues[3-fieldIndex]))
		}
		if fieldIndex <= 4 && 4 < fieldIndex+len(fieldValues) && len(fieldValues[4-fieldIndex]) > 0 {
			cond = append(cond, casbinrule.V4EQ(fieldValues[4-fieldIndex]))
		}
		if fieldIndex <= 5 && 5 < fieldIndex+len(fieldValues) && len(fieldValues[5-fieldIndex]) > 0 {
			cond = append(cond, casbinrule.V5EQ(fieldValues[5-fieldIndex]))
		}
		rules, err := tx.CasbinRule.Query().
			Where(cond...).
			All(a.ctx)
		if err != nil {
			return err
		}
		ruleIDs := make([]int, 0, len(rules))
		for _, r := range rules {
			ruleIDs = append(ruleIDs, r.ID)
		}

		_, err = tx.CasbinRule.Delete().
			Where(casbinrule.IDIn(ruleIDs...)).
			Exec(a.ctx)
		if err != nil {
			return err
		}

		if err := a.createPolicies(tx, ptype, newPolicies); err != nil {
			return err
		}
		for _, rule := range rules {
			oldPolicies = append(oldPolicies, CasbinRuleToStringArray(rule))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return oldPolicies, nil
}

func (a *Adapter) createPolicies(tx *ent.Tx, ptype string, policies [][]string) error {
	return a.insertPolicyLines(tx, policyLines(ptype, policies))
}

func CasbinRuleToStringArray(rule *ent.CasbinRule) []string {
	arr := make([]string, 0)
	if rule.V0 != "" {
		arr = append(arr, rule.V0)
	}
	if rule.V1 != "" {
		arr = append(arr, rule.V1)
	}
	if rule.V2 != "" {
		arr = append(arr, rule.V2)
	}
	if rule.V3 != "" {
		arr = append(arr, rule.V3)
	}
	if rule.V4 != "" {
		arr = append(arr, rule.V4)
	}
	if rule.V5 != "" {
		arr = append(arr, rule.V5)
	}
	return arr
}
