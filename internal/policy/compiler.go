package policy

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/goccy/go-yaml"
)

// PackFile is the on-disk YAML shape. A pack carries a slug, a version, and
// an ordered list of rules. The Action / Match fields map 1:1 onto the
// in-memory Rule type.
//
// Example pack:
//
//	name: default
//	version: 1
//	description: AgentGuard defaults
//	rules:
//	  - id: redact-aws-key
//	    match:
//	      content_regex: 'AKIA[0-9A-Z]{16}'
//	    action: redact
//	  - id: block-rm-rf
//	    match:
//	      tool: bash
//	      content_regex: 'rm\s+-rf\s+/'
//	    action: block
type PackFile struct {
	Name        string     `yaml:"name"`
	Version     int        `yaml:"version"`
	Description string     `yaml:"description"`
	Rules       []RuleSpec `yaml:"rules"`
}

// RuleSpec is one rule as written in YAML.
type RuleSpec struct {
	ID     string         `yaml:"id"`
	Match  MatchSpec      `yaml:"match"`
	Action string         `yaml:"action"`
	Params map[string]any `yaml:"params,omitempty"`
}

// MatchSpec is the predicate side of a rule.
type MatchSpec struct {
	Server       string `yaml:"server,omitempty"`
	Tool         string `yaml:"tool,omitempty"`
	Direction    string `yaml:"direction,omitempty"`
	ContentRegex string `yaml:"content_regex,omitempty"`
}

// CompilePack parses one YAML pack and returns the runtime Rule list.
func CompilePack(data []byte) (*PackFile, []Rule, error) {
	var pf PackFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, nil, fmt.Errorf("parse pack yaml: %w", err)
	}
	if pf.Name == "" {
		return nil, nil, errors.New("pack: missing name")
	}
	if pf.Version == 0 {
		return nil, nil, errors.New("pack: missing version")
	}
	rules := make([]Rule, 0, len(pf.Rules))
	for i, rs := range pf.Rules {
		r, err := compileRule(i, rs)
		if err != nil {
			return nil, nil, fmt.Errorf("pack %q rule %d (%s): %w", pf.Name, i, rs.ID, err)
		}
		rules = append(rules, r)
	}
	return &pf, rules, nil
}

func compileRule(idx int, rs RuleSpec) (Rule, error) {
	if rs.Action == "" {
		return Rule{}, errors.New("missing action")
	}
	act := Action(rs.Action)
	switch act {
	case ActionAllow, ActionBlock, ActionFlag, ActionRedact,
		ActionRequireApproval, ActionRateLimit:
	default:
		return Rule{}, fmt.Errorf("unknown action %q", rs.Action)
	}
	id := rs.ID
	if id == "" {
		id = fmt.Sprintf("rule-%d", idx)
	}
	r := Rule{
		ID:     id,
		Server: rs.Match.Server,
		Tool:   rs.Match.Tool,
		Action: act,
		Params: rs.Params,
	}
	switch rs.Match.Direction {
	case "", "any", "*":
		r.Direction = AnyDir
	case "outbound":
		r.Direction = Outbound
	case "inbound":
		r.Direction = Inbound
	default:
		return Rule{}, fmt.Errorf("unknown direction %q", rs.Match.Direction)
	}
	if rs.Match.ContentRegex != "" {
		re, err := regexp.Compile(rs.Match.ContentRegex)
		if err != nil {
			return Rule{}, fmt.Errorf("invalid content_regex: %w", err)
		}
		r.ContentMatch = re
	}
	return r, nil
}
