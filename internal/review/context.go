package review

import "strings"

// ExtractDiffContext returns surrounding changed lines for a comment target.
func ExtractDiffContext(filePath string, line int, diffLines []DiffLine, window int) []string {
	if window < 0 {
		window = 0
	}
	var fileLines []DiffLine
	for _, dl := range diffLines {
		if dl.FilePath == filePath {
			fileLines = append(fileLines, dl)
		}
	}
	if len(fileLines) == 0 {
		return []string{}
	}
	target := -1
	for i, dl := range fileLines {
		if (dl.NewLine != nil && *dl.NewLine == line) || (dl.OldLine != nil && *dl.OldLine == line) {
			target = i
			break
		}
	}
	if target < 0 {
		return []string{}
	}
	start := target - window
	if start < 0 {
		start = 0
	}
	end := target + window + 1
	if end > len(fileLines) {
		end = len(fileLines)
	}
	out := make([]string, 0, end-start)
	for _, dl := range fileLines[start:end] {
		prefix := " "
		if dl.LineType == "+" || dl.LineType == "-" {
			prefix = dl.LineType
		}
		out = append(out, prefix+strings.TrimRight(dl.Content, "\r\n"))
	}
	return out
}
