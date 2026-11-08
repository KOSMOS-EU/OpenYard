package metadata

import (
	"fmt"
	"strconv"
	"strings"
)

// ReferencePlan describes how file references (Aktenzeichen) are built level
// by level: one pattern per level, each a separator followed by {n} or {n:W}
// (W digits, zero-padded). The documented syntax of the file plan
// (folderviews/AKTENPLAN.md) is the default:
//
//	11                Sachgruppe 1       {n:2}
//	11.12             Sachgruppe 2       .{n:2}
//	11.12.01          Sachgruppe n       .{n:2}
//	11.12.01.03       Aktenschrank       .{n:2}
//	11.12.01.03-01    Akte               -{n:2}
//	11.12.01.03-01/1  Vorgang            /{n}
//	11.12.01.03-01/1#1 Register          #{n}
//
// A folder created below a code gets the next level's pattern appended with
// the lowest free number among its siblings.
type ReferencePlan []string

// DefaultReferencePlan is the documented file plan syntax.
var DefaultReferencePlan = ReferencePlan{"{n:2}", ".{n:2}", ".{n:2}", ".{n:2}", "-{n:2}", "/{n}", "#{n}"}

type planLevel struct {
	separator string
	width     int
}

func (p ReferencePlan) levels() ([]planLevel, error) {
	levels := make([]planLevel, 0, len(p))
	for position, pattern := range p {
		start := strings.Index(pattern, "{n")
		end := strings.Index(pattern, "}")
		if start < 0 || end < start {
			return nil, fmt.Errorf("reference plan level %d %q: expected {n} or {n:W}", position+1, pattern)
		}
		level := planLevel{separator: pattern[:start]}
		spec := pattern[start+2 : end]
		if spec != "" {
			width, err := strconv.Atoi(strings.TrimPrefix(spec, ":"))
			if err != nil || width < 1 {
				return nil, fmt.Errorf("reference plan level %d %q: bad width", position+1, pattern)
			}
			level.width = width
		}
		if position > 0 && level.separator == "" {
			return nil, fmt.Errorf("reference plan level %d %q: separator required", position+1, pattern)
		}
		levels = append(levels, level)
	}
	if len(levels) == 0 {
		return nil, fmt.Errorf("reference plan is empty")
	}
	return levels, nil
}

// Validate reports a malformed plan.
func (p ReferencePlan) Validate() error {
	_, err := p.levels()
	return err
}

// Depth returns the number of levels a code spans according to the plan,
// 0 when the code does not follow it.
func (p ReferencePlan) Depth(code string) int {
	levels, err := p.levels()
	if err != nil {
		return 0
	}
	rest := code
	depth := 0
	for _, level := range levels {
		if depth > 0 {
			if !strings.HasPrefix(rest, level.separator) {
				break
			}
			rest = rest[len(level.separator):]
		}
		digits := 0
		for digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '9' {
			digits++
		}
		if digits == 0 {
			return 0
		}
		rest = rest[digits:]
		depth++
		if rest == "" {
			return depth
		}
	}
	return 0
}

// Child formats the code of a child at the level below parent with number n.
// ok is false when the parent is not a valid code or already at the deepest level.
func (p ReferencePlan) Child(parent string, n int) (string, bool) {
	levels, err := p.levels()
	if err != nil {
		return "", false
	}
	depth := p.Depth(parent)
	if depth == 0 || depth >= len(levels) {
		return "", false
	}
	level := levels[depth]
	number := strconv.Itoa(n)
	if level.width > 0 {
		number = fmt.Sprintf("%0*d", level.width, n)
	}
	return parent + level.separator + number, true
}

// ChildNumber parses the number of a direct child code of parent; ok is
// false when code is not a direct child according to the plan.
func (p ReferencePlan) ChildNumber(parent, code string) (int, bool) {
	levels, err := p.levels()
	if err != nil {
		return 0, false
	}
	depth := p.Depth(parent)
	if depth == 0 || depth >= len(levels) {
		return 0, false
	}
	prefix := parent + levels[depth].separator
	if !strings.HasPrefix(code, prefix) {
		return 0, false
	}
	number, err := strconv.Atoi(code[len(prefix):])
	if err != nil || number < 0 {
		return 0, false
	}
	return number, true
}

// ParentCode returns the code one level up (11.16.13.01-01 → 11.16.13.01,
// 11.16 → 11), "" at the top or for codes outside the plan.
func (p ReferencePlan) ParentCode(code string) string {
	levels, err := p.levels()
	if err != nil {
		return ""
	}
	depth := p.Depth(code)
	if depth <= 1 {
		return ""
	}
	separator := levels[depth-1].separator
	if cut := strings.LastIndex(code, separator); cut > 0 {
		return code[:cut]
	}
	return ""
}

// ReferencePlanFor returns the namespace's reference plan, the documented
// default when none is configured.
func (c *Config) ReferencePlanFor(login string) ReferencePlan {
	ns := c.NamespaceForUser(login)
	if len(ns.ReferencePlan) == 0 {
		return DefaultReferencePlan
	}
	return ns.ReferencePlan
}
