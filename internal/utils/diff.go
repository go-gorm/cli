package utils

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

// PrintDiff renders a compact unified diff for before/after and writes to stdout.
func PrintDiff(path string, before, after []byte) {
	if bytes.Equal(before, after) {
		fmt.Fprintf(os.Stdout, "No changes for %s\n", path)
		return
	}
	delta := unifiedDiff(path, before, after)
	if strings.TrimSpace(delta) == "" {
		fmt.Fprintf(os.Stdout, "--- Proposed changes for %s ---\n%s\n--- End of changes ---\n", path, string(after))
		return
	}
	fmt.Fprintf(os.Stdout, "--- Proposed changes for %s ---\n%s\n--- End of changes ---\n", path, delta)
}

func unifiedDiff(path string, before, after []byte) string {
	split := func(b []byte) []string {
		if len(b) == 0 {
			return nil
		}
		lines := strings.SplitAfter(string(b), "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		return lines
	}

	type line struct {
		kind byte
		text string
	}
	type hunk struct {
		aStart int
		bStart int
		aLines int
		bLines int
		lines  []line
	}

	a := split(before)
	b := split(after)
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	var seq []line
	i, j := 0, 0
	for i < m && j < n {
		if a[i] == b[j] {
			seq = append(seq, line{' ', a[i]})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			seq = append(seq, line{'-', a[i]})
			i++
		} else {
			seq = append(seq, line{'+', b[j]})
			j++
		}
	}
	for i < m {
		seq = append(seq, line{'-', a[i]})
		i++
	}
	for j < n {
		seq = append(seq, line{'+', b[j]})
		j++
	}

	const context = 3
	var (
		leading      []line
		cur          *hunk
		hunks        []hunk
		aLine, bLine = 1, 1
		postContext  int
	)

	finish := func() {
		if cur == nil {
			return
		}
		hunks = append(hunks, *cur)
		cur = nil
		postContext = 0
		leading = nil
	}

	for _, ln := range seq {
		switch ln.kind {
		case ' ':
			if cur == nil {
				leading = append(leading, ln)
				if len(leading) > context {
					leading = leading[1:]
				}
			} else {
				cur.lines = append(cur.lines, ln)
				cur.aLines++
				cur.bLines++
				postContext++
				if postContext >= context {
					finish()
				}
			}
			aLine++
			bLine++
		default:
			if cur == nil {
				startA := aLine - len(leading)
				startB := bLine - len(leading)
				if startA < 1 {
					startA = 1
				}
				if startB < 1 {
					startB = 1
				}
				cur = &hunk{aStart: startA, bStart: startB}
				for _, l := range leading {
					cur.lines = append(cur.lines, l)
					cur.aLines++
					cur.bLines++
				}
			}
			cur.lines = append(cur.lines, ln)
			if ln.kind == '-' {
				cur.aLines++
				aLine++
			} else {
				cur.bLines++
				bLine++
			}
			postContext = 0
		}
	}
	finish()

	if len(hunks) == 0 {
		return ""
	}

	var buf strings.Builder
	fmt.Fprintf(&buf, "--- %s\n+++ %s\n", path, path)
	for _, h := range hunks {
		if h.aLines == 0 {
			h.aLines = 1
		}
		if h.bLines == 0 {
			h.bLines = 1
		}
		fmt.Fprintf(&buf, "@@ -%d,%d +%d,%d @@\n", h.aStart, h.aLines, h.bStart, h.bLines)
		for _, ln := range h.lines {
			buf.WriteByte(ln.kind)
			buf.WriteString(ln.text)
			if !strings.HasSuffix(ln.text, "\n") {
				buf.WriteByte('\n')
			}
		}
	}
	return buf.String()
}
