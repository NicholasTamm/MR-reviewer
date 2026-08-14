package config

import (
	"os"
	"regexp"
	"strings"
	"unicode"
)

var (
	envBraceRE       = regexp.MustCompile(`\{env:([A-Za-z_][A-Za-z0-9_]*)\}`)
	envDollarBraceRE = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
)

func ExpandEnv(s string) string {
	if s == "" || !strings.ContainsAny(s, "{$") {
		return s
	}
	out := envBraceRE.ReplaceAllStringFunc(s, func(m string) string {
		sub := envBraceRE.FindStringSubmatch(m)
		if len(sub) != 2 {
			return m
		}
		return os.Getenv(sub[1])
	})
	out = envDollarBraceRE.ReplaceAllStringFunc(out, func(m string) string {
		sub := envDollarBraceRE.FindStringSubmatch(m)
		if len(sub) != 2 {
			return m
		}
		return os.Getenv(sub[1])
	})
	return expandDollarVars(out)
}

func expandDollarVars(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '$' {
			b.WriteByte(s[i])
			continue
		}
		if i+1 < len(s) && s[i+1] == '{' {
			b.WriteByte('$')
			continue
		}
		j := i + 1
		if j < len(s) && (s[j] == '_' || unicode.IsLetter(rune(s[j]))) {
			j++
			for j < len(s) && (s[j] == '_' || unicode.IsLetter(rune(s[j])) || unicode.IsDigit(rune(s[j]))) {
				j++
			}
			b.WriteString(os.Getenv(s[i+1 : j]))
			i = j - 1
			continue
		}
		b.WriteByte('$')
	}
	return b.String()
}

func ContainsEnvRef(s string) bool {
	if s == "" {
		return false
	}
	if envBraceRE.MatchString(s) || envDollarBraceRE.MatchString(s) {
		return true
	}
	for i := 0; i < len(s); i++ {
		if s[i] != '$' || i+1 >= len(s) {
			continue
		}
		c := s[i+1]
		if c == '_' || unicode.IsLetter(rune(c)) {
			return true
		}
	}
	return false
}

func EnvRefName(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	if m := envBraceRE.FindStringSubmatch(s); len(m) == 2 && m[0] == s {
		return m[1], true
	}
	if m := envDollarBraceRE.FindStringSubmatch(s); len(m) == 2 && m[0] == s {
		return m[1], true
	}
	if strings.HasPrefix(s, "$") && !strings.HasPrefix(s, "${") {
		name := s[1:]
		if isEnvIdent(name) {
			return name, true
		}
	}
	return "", false
}

func isEnvIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func NormalizeAPIKeyEnv(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if name, ok := EnvRefName(raw); ok {
		return name
	}
	return raw
}
