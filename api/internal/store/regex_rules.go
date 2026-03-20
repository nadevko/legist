package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// RegexWeightRule is one embedding-weight classification rule.
// If multiple enabled rules match, the embedder takes max(weight).
type RegexWeightRule struct {
	ID        string    `db:"id"`
	Regex     string    `db:"regex"`
	Enabled   bool      `db:"enabled"`
	Weight    float64   `db:"weight"`
	CreatedAt time.Time `db:"created_at"`
}

type RegexWeightRuleCreate struct {
	Regex   string
	Enabled bool
	Weight  float64
}

type RegexWeightRuleUpdate struct {
	Regex   *string
	Enabled *bool
	Weight  *float64
}

// RegexOmitRule is one Stage3 risk-zone omit rule.
// If at least one enabled omit rule matches a changed fragment, the match is nulled.
type RegexOmitRule struct {
	ID        string    `db:"id"`
	Regex     string    `db:"regex"`
	Enabled   bool      `db:"enabled"`
	CreatedAt time.Time `db:"created_at"`
}

type RegexOmitRuleCreate struct {
	Regex   string
	Enabled bool
}

type RegexOmitRuleUpdate struct {
	Regex   *string
	Enabled *bool
}

type RegexRulesStore struct{ db *sqlx.DB }

func NewRegexRulesStore(db *sqlx.DB) *RegexRulesStore { return &RegexRulesStore{db: db} }

// --- weights ---

func (s *RegexRulesStore) ListWeightRules() ([]RegexWeightRule, error) {
	var out []RegexWeightRule
	if err := s.db.Select(&out, `
		SELECT id, regex, enabled, weight, created_at
		FROM regex_weight_rules
		ORDER BY created_at ASC, id ASC
	`); err != nil {
		return nil, fmt.Errorf("list weight rules: %w", err)
	}
	return out, nil
}

func (s *RegexRulesStore) GetWeightRule(id string) (*RegexWeightRule, error) {
	var r RegexWeightRule
	if err := s.db.Get(&r, `
		SELECT id, regex, enabled, weight, created_at
		FROM regex_weight_rules
		WHERE id = ?
	`, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("get weight rule: %w", err)
	}
	return &r, nil
}

func (s *RegexRulesStore) CreateWeightRule(id string, in RegexWeightRuleCreate) error {
	_, err := s.db.Exec(`
		INSERT INTO regex_weight_rules (id, regex, enabled, weight)
		VALUES (?, ?, ?, ?)
	`, id, in.Regex, boolToInt(in.Enabled), in.Weight)
	if err != nil {
		return fmt.Errorf("create weight rule: %w", err)
	}
	return nil
}

func (s *RegexRulesStore) UpdateWeightRule(id string, in RegexWeightRuleUpdate) error {
	// Allow partial updates; build SET clause dynamically.
	set := ""
	args := []any{}
	if in.Regex != nil {
		set += ", regex = ?"
		args = append(args, *in.Regex)
	}
	if in.Enabled != nil {
		set += ", enabled = ?"
		args = append(args, boolToInt(*in.Enabled))
	}
	if in.Weight != nil {
		set += ", weight = ?"
		args = append(args, *in.Weight)
	}
	if set == "" {
		return nil
	}
	// drop leading comma
	set = set[1:]
	args = append(args, id)

	q := fmt.Sprintf(`UPDATE regex_weight_rules SET %s WHERE id = ?`, set)
	if _, err := s.db.Exec(q, args...); err != nil {
		return fmt.Errorf("update weight rule: %w", err)
	}
	return nil
}

func (s *RegexRulesStore) DeleteWeightRule(id string) error {
	if _, err := s.db.Exec(`DELETE FROM regex_weight_rules WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete weight rule: %w", err)
	}
	return nil
}

func (s *RegexRulesStore) ReplaceWeightRules(rules []RegexWeightRuleCreate, ids []string) error {
	// ids is optional: if provided, must match rules length.
	if ids != nil && len(ids) != len(rules) {
		return fmt.Errorf("replace weight rules: ids length mismatch")
	}

	tx, err := s.db.Beginx()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM regex_weight_rules`); err != nil {
		return fmt.Errorf("delete weight rules: %w", err)
	}

	for i := range rules {
		id := newID("wrule")
		if ids != nil {
			id = ids[i]
		}
		if _, err := tx.Exec(`
			INSERT INTO regex_weight_rules (id, regex, enabled, weight)
			VALUES (?, ?, ?, ?)
		`, id, rules[i].Regex, boolToInt(rules[i].Enabled), rules[i].Weight); err != nil {
			return fmt.Errorf("insert weight rule: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// --- omits ---

func (s *RegexRulesStore) ListOmitRules() ([]RegexOmitRule, error) {
	var out []RegexOmitRule
	if err := s.db.Select(&out, `
		SELECT id, regex, enabled, created_at
		FROM regex_omit_rules
		ORDER BY created_at ASC, id ASC
	`); err != nil {
		return nil, fmt.Errorf("list omit rules: %w", err)
	}
	return out, nil
}

func (s *RegexRulesStore) GetOmitRule(id string) (*RegexOmitRule, error) {
	var r RegexOmitRule
	if err := s.db.Get(&r, `
		SELECT id, regex, enabled, created_at
		FROM regex_omit_rules
		WHERE id = ?
	`, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("get omit rule: %w", err)
	}
	return &r, nil
}

func (s *RegexRulesStore) CreateOmitRule(id string, in RegexOmitRuleCreate) error {
	_, err := s.db.Exec(`
		INSERT INTO regex_omit_rules (id, regex, enabled)
		VALUES (?, ?, ?)
	`, id, in.Regex, boolToInt(in.Enabled))
	if err != nil {
		return fmt.Errorf("create omit rule: %w", err)
	}
	return nil
}

func (s *RegexRulesStore) UpdateOmitRule(id string, in RegexOmitRuleUpdate) error {
	set := ""
	args := []any{}
	if in.Regex != nil {
		set += ", regex = ?"
		args = append(args, *in.Regex)
	}
	if in.Enabled != nil {
		set += ", enabled = ?"
		args = append(args, boolToInt(*in.Enabled))
	}
	if set == "" {
		return nil
	}
	set = set[1:]
	args = append(args, id)

	q := fmt.Sprintf(`UPDATE regex_omit_rules SET %s WHERE id = ?`, set)
	if _, err := s.db.Exec(q, args...); err != nil {
		return fmt.Errorf("update omit rule: %w", err)
	}
	return nil
}

func (s *RegexRulesStore) DeleteOmitRule(id string) error {
	if _, err := s.db.Exec(`DELETE FROM regex_omit_rules WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete omit rule: %w", err)
	}
	return nil
}

func (s *RegexRulesStore) ReplaceOmitRules(rules []RegexOmitRuleCreate, ids []string) error {
	if ids != nil && len(ids) != len(rules) {
		return fmt.Errorf("replace omit rules: ids length mismatch")
	}

	tx, err := s.db.Beginx()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM regex_omit_rules`); err != nil {
		return fmt.Errorf("delete omit rules: %w", err)
	}

	for i := range rules {
		id := newID("orule")
		if ids != nil {
			id = ids[i]
		}
		if _, err := tx.Exec(`
			INSERT INTO regex_omit_rules (id, regex, enabled)
			VALUES (?, ?, ?)
		`, id, rules[i].Regex, boolToInt(rules[i].Enabled)); err != nil {
			return fmt.Errorf("insert omit rule: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// newID is used only as a local helper during Replace*.
// For API-level POST/PATCH/DELETE we can generate ids in handlers too,
// but having it here keeps Replace* self-contained.
func newID(prefix string) string {
	raw := strings.ReplaceAll(uuid.NewString(), "-", "")
	return prefix + "_" + raw[:12]
}

func (s *RegexRulesStore) SeedFromTemplatesIfEmpty(weightTemplatePath, omitTemplatePath string) error {
	weightCount, err := s.countWeightRules()
	if err != nil {
		return err
	}
	if weightCount == 0 && strings.TrimSpace(weightTemplatePath) != "" {
		rules, err := loadWeightTemplateJSON(weightTemplatePath)
		if err != nil {
			return fmt.Errorf("seed weight rules: %w", err)
		}
		if len(rules) > 0 {
			for i, r := range rules {
				if _, err := regexp.Compile(r.Regex); err != nil {
					return fmt.Errorf("invalid seed weight regex: index=%d regex=%q: %w", i, r.Regex, err)
				}
			}
			if err := s.ReplaceWeightRules(rules, nil); err != nil {
				return fmt.Errorf("seed weight rules insert: %w", err)
			}
		}
	}

	omitCount, err := s.countOmitRules()
	if err != nil {
		return err
	}
	if omitCount == 0 && strings.TrimSpace(omitTemplatePath) != "" {
		rules, err := loadOmitTemplateJSON(omitTemplatePath)
		if err != nil {
			return fmt.Errorf("seed omit rules: %w", err)
		}
		if len(rules) > 0 {
			for i, r := range rules {
				if _, err := regexp.Compile(r.Regex); err != nil {
					return fmt.Errorf("invalid seed omit regex: index=%d regex=%q: %w", i, r.Regex, err)
				}
			}
			if err := s.ReplaceOmitRules(rules, nil); err != nil {
				return fmt.Errorf("seed omit rules insert: %w", err)
			}
		}
	}

	return nil
}

// ResetWeightRulesFromTemplate replaces all weight rules with those from the given JSON template.
// If templatePath is empty, resets to an empty rule set.
func (s *RegexRulesStore) ResetWeightRulesFromTemplate(templatePath string) error {
	var rules []RegexWeightRuleCreate
	if strings.TrimSpace(templatePath) != "" {
		var err error
		rules, err = loadWeightTemplateJSON(templatePath)
		if err != nil {
			return fmt.Errorf("load weight template: %w", err)
		}
	}
	for _, r := range rules {
		if _, err := regexp.Compile(r.Regex); err != nil {
			return fmt.Errorf("invalid weight regex in template: %w", err)
		}
	}
	return s.ReplaceWeightRules(rules, nil)
}

// ResetOmitRulesFromTemplate replaces all omit rules with those from the given JSON template.
// If templatePath is empty, resets to an empty rule set.
func (s *RegexRulesStore) ResetOmitRulesFromTemplate(templatePath string) error {
	var rules []RegexOmitRuleCreate
	if strings.TrimSpace(templatePath) != "" {
		var err error
		rules, err = loadOmitTemplateJSON(templatePath)
		if err != nil {
			return fmt.Errorf("load omit template: %w", err)
		}
	}
	for _, r := range rules {
		if _, err := regexp.Compile(r.Regex); err != nil {
			return fmt.Errorf("invalid omit regex in template: %w", err)
		}
	}
	return s.ReplaceOmitRules(rules, nil)
}

func (s *RegexRulesStore) countWeightRules() (int, error) {
	var n int
	if err := s.db.Get(&n, `SELECT COUNT(*) FROM regex_weight_rules`); err != nil {
		return 0, fmt.Errorf("count weight rules: %w", err)
	}
	return n, nil
}

func (s *RegexRulesStore) countOmitRules() (int, error) {
	var n int
	if err := s.db.Get(&n, `SELECT COUNT(*) FROM regex_omit_rules`); err != nil {
		return 0, fmt.Errorf("count omit rules: %w", err)
	}
	return n, nil
}

type weightTemplateItem struct {
	Regex   string   `json:"regex"`
	Enabled *bool    `json:"enabled"`
	Weight  float64  `json:"weight"`
}

func loadWeightTemplateJSON(path string) ([]RegexWeightRuleCreate, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	var items []weightTemplateItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("unmarshal json %q: %w", path, err)
	}
	out := make([]RegexWeightRuleCreate, 0, len(items))
	for _, it := range items {
		if strings.TrimSpace(it.Regex) == "" {
			continue
		}
		enabled := true
		if it.Enabled != nil {
			enabled = *it.Enabled
		}
		out = append(out, RegexWeightRuleCreate{
			Regex:   it.Regex,
			Enabled: enabled,
			Weight:  it.Weight,
		})
	}
	return out, nil
}

type omitTemplateItem struct {
	Regex   string `json:"regex"`
	Enabled *bool  `json:"enabled"`
}

func loadOmitTemplateJSON(path string) ([]RegexOmitRuleCreate, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	var items []omitTemplateItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("unmarshal json %q: %w", path, err)
	}
	out := make([]RegexOmitRuleCreate, 0, len(items))
	for _, it := range items {
		if strings.TrimSpace(it.Regex) == "" {
			continue
		}
		enabled := true
		if it.Enabled != nil {
			enabled = *it.Enabled
		}
		out = append(out, RegexOmitRuleCreate{
			Regex:   it.Regex,
			Enabled: enabled,
		})
	}
	return out, nil
}

