package review

import (
	"regexp"
	"strconv"
	"strings"
)

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

func stripAB(path string) string {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "a/") || strings.HasPrefix(path, "b/") {
		return path[2:]
	}
	return path
}

// ParseDiff returns added and removed lines (not context) from a unified diff.
func ParseDiff(diff string) []DiffLine {
	if strings.TrimSpace(diff) == "" {
		return nil
	}
	var lines []DiffLine
	var filePath string
	var oldLine, newLine int
	sawHunk := false

	for _, raw := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(raw, "+++ "):
			filePath = stripAB(strings.TrimPrefix(raw, "+++ "))
		case strings.HasPrefix(raw, "--- "):
			if filePath == "" {
				filePath = stripAB(strings.TrimPrefix(raw, "--- "))
			}
		case strings.HasPrefix(raw, "@@"):
			m := hunkHeader.FindStringSubmatch(raw)
			if m == nil {
				continue
			}
			sawHunk = true
			oldLine, _ = strconv.Atoi(m[1])
			newLine, _ = strconv.Atoi(m[2])
		case strings.HasPrefix(raw, "diff ") || strings.HasPrefix(raw, "index ") ||
			strings.HasPrefix(raw, "new file") || strings.HasPrefix(raw, "deleted file") ||
			strings.HasPrefix(raw, "rename ") || strings.HasPrefix(raw, "similarity "):
			continue
		case strings.HasPrefix(raw, "\\"):
			continue
		case strings.HasPrefix(raw, "+"):
			if !sawHunk {
				continue
			}
			nl := newLine
			lines = append(lines, DiffLine{
				FilePath: filePath,
				NewLine:  &nl,
				LineType: "+",
				Content:  raw[1:],
			})
			newLine++
		case strings.HasPrefix(raw, "-"):
			if !sawHunk {
				continue
			}
			ol := oldLine
			lines = append(lines, DiffLine{
				FilePath: filePath,
				OldLine:  &ol,
				LineType: "-",
				Content:  raw[1:],
			})
			oldLine++
		default:
			if sawHunk && !strings.HasPrefix(raw, "---") && !strings.HasPrefix(raw, "+++") {
				oldLine++
				newLine++
			}
		}
	}
	return lines
}

// ChangedPaths returns unique new-file paths from a unified diff.
func ChangedPaths(diff string) []string {
	seen := map[string]struct{}{}
	var paths []string
	for _, raw := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(raw, "+++ ") {
			continue
		}
		path := stripAB(strings.TrimPrefix(raw, "+++ "))
		if path == "/dev/null" || path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}

// AnnotateDiff prefixes each changed line with [L{n}] using hunk counters.
func AnnotateDiff(diff string) string {
	var b strings.Builder
	oldLine, newLine := 0, 0
	parts := strings.Split(diff, "\n")
	for i, raw := range parts {
		switch {
		case strings.HasPrefix(raw, "@@"):
			m := hunkHeader.FindStringSubmatch(raw)
			if m != nil {
				oldLine, _ = strconv.Atoi(m[1])
				newLine, _ = strconv.Atoi(m[2])
			}
			b.WriteString(raw)
		case strings.HasPrefix(raw, "+") && !strings.HasPrefix(raw, "+++"):
			b.WriteString("+[L")
			b.WriteString(strconv.Itoa(newLine))
			b.WriteString("]")
			b.WriteString(raw[1:])
			newLine++
		case strings.HasPrefix(raw, "-") && !strings.HasPrefix(raw, "---"):
			b.WriteString("-[L")
			b.WriteString(strconv.Itoa(oldLine))
			b.WriteString("]")
			b.WriteString(raw[1:])
			oldLine++
		default:
			if !strings.HasPrefix(raw, "---") && !strings.HasPrefix(raw, "+++") {
				oldLine++
				newLine++
			}
			b.WriteString(raw)
		}
		if i < len(parts)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// ValidateCommentLine is true when (file, line) is an added line in the diff.
func ValidateCommentLine(filePath string, line int, diffLines []DiffLine) bool {
	for _, dl := range diffLines {
		if dl.FilePath == filePath && dl.LineType == "+" && dl.NewLine != nil && *dl.NewLine == line {
			return true
		}
	}
	return false
}

// DetermineNewLine is true when the comment targets an addition (default true).
func DetermineNewLine(filePath string, line int, diffLines []DiffLine) bool {
	for _, dl := range diffLines {
		if dl.FilePath != filePath {
			continue
		}
		if dl.NewLine != nil && *dl.NewLine == line && dl.LineType == "+" {
			return true
		}
		if dl.OldLine != nil && *dl.OldLine == line && dl.LineType == "-" {
			return false
		}
	}
	return true
}

// BuildUnifiedDiff joins per-file patches into one unified diff.
func BuildUnifiedDiff(files []DiffFile) string {
	var parts []string
	for _, df := range files {
		if df.IsBinary || df.Diff == "" {
			continue
		}
		parts = append(parts, "--- a/"+df.OldPath+"\n+++ b/"+df.NewPath+"\n"+df.Diff)
	}
	return strings.Join(parts, "\n")
}
