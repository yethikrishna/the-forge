// Package grammar audits and enforces unified command grammar.
// Ensures all forge commands follow `forge <noun> <verb>` pattern.
// Detects violations and suggests corrections.
//
// Consistency is not limiting — it's liberating.
package grammar

import (
	"fmt"
	"regexp"
	"strings"
)

// Violation represents a command that violates the grammar rules.
type Violation struct {
	Command     string   `json:"command"`
	Expected    string   `json:"expected"`
	Reason      string   `json:"reason"`
	Suggestions []string `json:"suggestions"`
}

// Pattern represents a valid command pattern.
type Pattern struct {
	Noun    string   // e.g. "prompt", "session", "agent"
	Verbs   []string // e.g. "list", "get", "create", "delete"
	Aliases []string // e.g. "ls" → "list"
}

// Auditor audits command grammar.
type Auditor struct {
	patterns  map[string]*Pattern
	overrides map[string]string // legacy → canonical
}

// NewAuditor creates a grammar auditor with standard patterns.
func NewAuditor() *Auditor {
	a := &Auditor{
		patterns:  make(map[string]*Pattern),
		overrides: make(map[string]string),
	}
	a.registerDefaults()
	return a
}

func (a *Auditor) registerDefaults() {
	// Standard forge <noun> <verb> patterns
	a.RegisterPattern(Pattern{Noun: "prompt", Verbs: []string{"list", "get", "create", "delete", "edit", "test", "analyze"}})
	a.RegisterPattern(Pattern{Noun: "session", Verbs: []string{"list", "get", "create", "delete", "resume", "archive"}})
	a.RegisterPattern(Pattern{Noun: "agent", Verbs: []string{"list", "get", "create", "delete", "run", "stop", "status"}})
	a.RegisterPattern(Pattern{Noun: "role", Verbs: []string{"list", "get", "assign", "can"}})
	a.RegisterPattern(Pattern{Noun: "scope", Verbs: []string{"set", "check", "list", "remove"}})
	a.RegisterPattern(Pattern{Noun: "scan", Verbs: []string{"pre", "post", "history"}})
	a.RegisterPattern(Pattern{Noun: "test", Verbs: []string{"run", "list", "create", "report"}})
	a.RegisterPattern(Pattern{Noun: "config", Verbs: []string{"get", "set", "list", "validate"}})
	a.RegisterPattern(Pattern{Noun: "trust", Verbs: []string{"list", "get", "score", "history"}})
	a.RegisterPattern(Pattern{Noun: "preview", Verbs: []string{"create", "approve", "reject", "list", "show", "restore"}})
	a.RegisterPattern(Pattern{Noun: "approval", Verbs: []string{"list", "request", "accept", "deny", "escalate", "resolved"}})

	// Verb aliases
	a.RegisterAlias("ls", "list")
	a.RegisterAlias("rm", "delete")
	a.RegisterAlias("add", "create")
	a.RegisterAlias("new", "create")
	a.RegisterAlias("start", "run")
	a.RegisterAlias("info", "get")
	a.RegisterAlias("show", "get")

	// Legacy overrides: verb-first → noun-verb
	a.RegisterOverride("run", "agent run")
	a.RegisterOverride("doctor", "forge doctor") // top-level exception
	a.RegisterOverride("watch", "forge watch")   // top-level exception
	a.RegisterOverride("share", "forge share")
	a.RegisterOverride("undo", "forge undo")
	a.RegisterOverride("explain", "forge explain")
	a.RegisterOverride("complete", "forge completion")
}

// RegisterPattern adds a valid command pattern.
func (a *Auditor) RegisterPattern(p Pattern) {
	a.patterns[p.Noun] = &p
}

// RegisterAlias maps a verb alias to its canonical form.
func (a *Auditor) RegisterAlias(alias, canonical string) {
	for _, p := range a.patterns {
		for _, v := range p.Verbs {
			if v == canonical {
				// Check not already present
				found := false
				for _, existing := range p.Aliases {
					if existing == alias {
						found = true
						break
					}
				}
				if !found {
					p.Aliases = append(p.Aliases, alias)
				}
			}
		}
	}
}

// RegisterOverride maps a legacy command to its canonical form.
func (a *Auditor) RegisterOverride(legacy, canonical string) {
	a.overrides[legacy] = canonical
}

// Audit checks a command string against grammar rules.
func (a *Auditor) Audit(command string) []Violation {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}

	var violations []Violation

	// Top-level exceptions (single word commands like "doctor", "watch", "version")
	exceptions := map[string]bool{
		"doctor": true, "watch": true, "version": true, "help": true,
		"completion": true, "level": true, "overview": true, "quickstart": true,
	}

	parts := strings.Fields(command)

	// Skip "forge" prefix if present
	if len(parts) > 0 && parts[0] == "forge" {
		parts = parts[1:]
	}

	if len(parts) == 0 {
		return nil
	}

	// Single-word commands — check exceptions
	if len(parts) == 1 {
		if exceptions[parts[0]] {
			return nil
		}
		if canonical, ok := a.overrides[parts[0]]; ok {
			violations = append(violations, Violation{
				Command:     command,
				Expected:    canonical,
				Reason:      fmt.Sprintf("'%s' should use noun-verb form", parts[0]),
				Suggestions: []string{canonical},
			})
		}
		return violations
	}

	noun := parts[0]
	verb := parts[1]

	// Check if noun is known
	pattern, known := a.patterns[noun]
	if !known {
		// Might be a verb-first command
		if canonical, ok := a.overrides[noun]; ok {
			violations = append(violations, Violation{
				Command:     command,
				Expected:    canonical + " " + strings.Join(parts[1:], " "),
				Reason:      fmt.Sprintf("'%s' is a legacy verb-first command", noun),
				Suggestions: []string{canonical + " " + strings.Join(parts[1:], " ")},
			})
		}
		return violations
	}

	// Check verb is valid for this noun
	valid := false
	for _, v := range pattern.Verbs {
		if v == verb {
			valid = true
			break
		}
	}

	// Check aliases
	if !valid {
		for _, alias := range pattern.Aliases {
			if alias == verb {
				// Alias is understood but non-canonical
				violations = append(violations, Violation{
					Command:     command,
					Expected:    noun + " " + resolveAlias(pattern, verb),
					Reason:      fmt.Sprintf("'%s' is an alias — use canonical verb '%s'", verb, resolveAlias(pattern, verb)),
					Suggestions: []string{noun + " " + resolveAlias(pattern, verb)},
				})
				return violations
			}
		}

		// Unknown verb
		violations = append(violations, Violation{
			Command:     command,
			Expected:    noun + " <verb>",
			Reason:      fmt.Sprintf("'%s' is not a valid verb for '%s'", verb, noun),
			Suggestions: pattern.Verbs,
		})
	}

	return violations
}

// AuditAll checks multiple commands and returns all violations.
func (a *Auditor) AuditAll(commands []string) []Violation {
	var all []Violation
	for _, cmd := range commands {
		all = append(all, a.Audit(cmd)...)
	}
	return all
}

// Report generates a human-readable audit report.
func (a *Auditor) Report(commands []string) string {
	violations := a.AuditAll(commands)
	if len(violations) == 0 {
		return "All commands follow unified grammar.\n"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Grammar Audit: %d violation(s)\n\n", len(violations)))
	for i, v := range violations {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, v.Command))
		b.WriteString(fmt.Sprintf("   Issue: %s\n", v.Reason))
		if v.Expected != "" {
			b.WriteString(fmt.Sprintf("   Expected: %s\n", v.Expected))
		}
		if len(v.Suggestions) > 0 {
			b.WriteString(fmt.Sprintf("   Suggestions: %s\n", strings.Join(v.Suggestions, ", ")))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// IsCanonical checks if a command uses canonical grammar.
func (a *Auditor) IsCanonical(command string) bool {
	return len(a.Audit(command)) == 0
}

// Nouns returns all registered nouns.
func (a *Auditor) Nouns() []string {
	nouns := make([]string, 0, len(a.patterns))
	for n := range a.patterns {
		nouns = append(nouns, n)
	}
	return nouns
}

// VerbsFor returns valid verbs for a noun.
func (a *Auditor) VerbsFor(noun string) []string {
	p, ok := a.patterns[noun]
	if !ok {
		return nil
	}
	return p.Verbs
}

func resolveAlias(p *Pattern, alias string) string {
	// Map common aliases
	aliasMap := map[string]string{
		"ls": "list", "rm": "delete", "add": "create", "new": "create",
		"start": "run", "info": "get", "show": "get",
	}
	if canonical, ok := aliasMap[alias]; ok {
		// Verify canonical is in verbs
		for _, v := range p.Verbs {
			if v == canonical {
				return canonical
			}
		}
	}
	return alias
}

// CommandRegex validates command format.
var commandRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// ValidateName checks if a command name follows conventions.
func ValidateName(name string) error {
	if !commandRegex.MatchString(name) {
		return fmt.Errorf("command name '%s' must be lowercase alphanumeric with hyphens", name)
	}
	return nil
}
