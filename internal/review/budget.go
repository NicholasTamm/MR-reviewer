package review

import "sort"

var severityPriority = map[string]int{
	"warning": 0,
	"info":    1,
}

// EnforceBudget keeps every error-severity comment and fills the remaining
// slots with warnings before info. Result is sorted by file, then line.
func EnforceBudget(comments []Comment, maxComments int) []Comment {
	var critical, rest []Comment
	for _, c := range comments {
		if c.Severity == "error" {
			critical = append(critical, c)
		} else {
			rest = append(rest, c)
		}
	}
	budget := maxComments - len(critical)
	if budget < 0 {
		budget = 0
	}
	if len(rest) > budget {
		sort.SliceStable(rest, func(i, j int) bool {
			pi := severityPriority[rest[i].Severity]
			pj := severityPriority[rest[j].Severity]
			if pi != pj {
				return pi < pj
			}
			return false
		})
		rest = rest[:budget]
	}
	out := append(critical, rest...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}
