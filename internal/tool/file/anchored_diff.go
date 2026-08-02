// Portions of this file are adapted from the Go Authors' anchored diff
// implementation in github.com/rogpeppe/go-internal/diff.
// Copyright 2022 The Go Authors. All rights reserved.
// See THIRD_PARTY_NOTICES.md for the applicable BSD license.

package file

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

const (
	diffContextLines = 3
	// 该估算覆盖行引用、哈希表、匹配数组和 hunk 计划；超过预算后切换到线性回退，
	// 不再创建与行数成比例的辅助结构。
	maxAnchoredDiffWorkingBytes   = 64 << 20
	estimatedAnchoredBytesPerLine = 192
	missingDiffLineNewlineMarker  = "\n\\ No newline at end of file\n"
)

type diffAlgorithm uint8

const (
	diffAlgorithmAnchored diffAlgorithm = iota
	diffAlgorithmLinear
)

type diffLine struct {
	text       string
	terminated bool
}

type diffPair struct {
	old int
	new int
}

type diffSpan struct {
	prefix byte
	start  int
	end    int
}

type diffHunk struct {
	oldStart  int
	oldCount  int
	newStart  int
	newCount  int
	spanStart int
	spanEnd   int
}

type anchoredDiffPlan struct {
	hunks []diffHunk
	spans []diffSpan
	stats diffStats
}

func writeTextDiff(writer io.Writer, path, oldContent, newContent string) (diffStats, error) {
	oldLineCount := countTextDiffLines(oldContent)
	newLineCount := countTextDiffLines(newContent)
	if selectDiffAlgorithm(oldLineCount, newLineCount) == diffAlgorithmLinear {
		return writeLinearTextDiff(writer, path, oldContent, newContent, oldLineCount, newLineCount)
	}
	return writeAnchoredTextDiff(writer, path, oldContent, newContent, oldLineCount, newLineCount)
}

func selectDiffAlgorithm(oldLineCount, newLineCount int) diffAlgorithm {
	maxLines := maxAnchoredDiffWorkingBytes / estimatedAnchoredBytesPerLine
	if oldLineCount > maxLines || newLineCount > maxLines-oldLineCount {
		return diffAlgorithmLinear
	}
	return diffAlgorithmAnchored
}

func writeAnchoredTextDiff(writer io.Writer, path, oldContent, newContent string, oldLineCount, newLineCount int) (diffStats, error) {
	oldLines := splitTextDiffLines(oldContent, oldLineCount)
	newLines := splitTextDiffLines(newContent, newLineCount)
	plan := buildAnchoredDiffPlan(oldLines, newLines, anchoredDiffMatches(oldLines, newLines))

	if err := writeDiffPreamble(writer, path); err != nil {
		return diffStats{}, err
	}
	for _, hunk := range plan.hunks {
		if err := writeDiffHunkHeader(writer, hunk.oldStart, hunk.oldCount, hunk.newStart, hunk.newCount); err != nil {
			return diffStats{}, err
		}
		for _, span := range plan.spans[hunk.spanStart:hunk.spanEnd] {
			lines := oldLines
			if span.prefix == '+' {
				lines = newLines
			}
			if err := writeDiffLines(writer, span.prefix, lines[span.start:span.end]); err != nil {
				return diffStats{}, err
			}
		}
	}
	return plan.stats, nil
}

func buildAnchoredDiffPlan(oldLines, newLines []diffLine, matches []diffPair) anchoredDiffPlan {
	plan := anchoredDiffPlan{stats: diffStats{FilesChanged: 1}}
	var (
		done      diffPair
		chunk     diffPair
		count     diffPair
		spanStart int
	)

	for _, match := range matches {
		if match.old < done.old {
			continue
		}

		start := match
		for start.old > done.old && start.new > done.new && oldLines[start.old-1] == newLines[start.new-1] {
			start.old--
			start.new--
		}
		end := match
		for end.old < len(oldLines) && end.new < len(newLines) && oldLines[end.old] == newLines[end.new] {
			end.old++
			end.new++
		}

		appendDiffSpan(&plan, spanStart, '-', done.old, start.old)
		count.old += start.old - done.old
		plan.stats.Deletions += start.old - done.old
		appendDiffSpan(&plan, spanStart, '+', done.new, start.new)
		count.new += start.new - done.new
		plan.stats.Insertions += start.new - done.new

		commonLines := end.old - start.old
		if (end.old < len(oldLines) || end.new < len(newLines)) &&
			(commonLines < diffContextLines || (len(plan.spans) > spanStart && commonLines < 2*diffContextLines)) {
			appendDiffSpan(&plan, spanStart, ' ', start.old, end.old)
			count.old += commonLines
			count.new += commonLines
			done = end
			continue
		}

		if len(plan.spans) > spanStart {
			contextLines := commonLines
			if contextLines > diffContextLines {
				contextLines = diffContextLines
			}
			appendDiffSpan(&plan, spanStart, ' ', start.old, start.old+contextLines)
			count.old += contextLines
			count.new += contextLines
			done = diffPair{old: start.old + contextLines, new: start.new + contextLines}

			plan.hunks = append(plan.hunks, diffHunk{
				oldStart:  unifiedDiffRangeStart(chunk.old, count.old),
				oldCount:  count.old,
				newStart:  unifiedDiffRangeStart(chunk.new, count.new),
				newCount:  count.new,
				spanStart: spanStart,
				spanEnd:   len(plan.spans),
			})
			count = diffPair{}
			spanStart = len(plan.spans)
		}

		if end.old >= len(oldLines) && end.new >= len(newLines) {
			break
		}

		chunk = diffPair{old: end.old - diffContextLines, new: end.new - diffContextLines}
		appendDiffSpan(&plan, spanStart, ' ', chunk.old, end.old)
		count.old += diffContextLines
		count.new += diffContextLines
		done = end
	}
	return plan
}

func appendDiffSpan(plan *anchoredDiffPlan, hunkSpanStart int, prefix byte, start, end int) {
	if start >= end {
		return
	}
	if len(plan.spans) > hunkSpanStart {
		last := &plan.spans[len(plan.spans)-1]
		if last.prefix == prefix && last.end == start {
			last.end = end
			return
		}
	}
	plan.spans = append(plan.spans, diffSpan{prefix: prefix, start: start, end: end})
}

func anchoredDiffMatches(oldLines, newLines []diffLine) []diffPair {
	counts := make(map[diffLine]int)
	for _, line := range oldLines {
		if count := counts[line]; count > -2 {
			counts[line] = count - 1
		}
	}
	for _, line := range newLines {
		if count := counts[line]; count > -8 {
			counts[line] = count - 4
		}
	}

	var oldIndexes, newIndexes, inverse []int
	for index, line := range newLines {
		if counts[line] == -5 {
			counts[line] = len(newIndexes)
			newIndexes = append(newIndexes, index)
		}
	}
	for index, line := range oldLines {
		if inverseIndex, exists := counts[line]; exists && inverseIndex >= 0 {
			oldIndexes = append(oldIndexes, index)
			inverse = append(inverse, inverseIndex)
		}
	}

	uniqueCount := len(oldIndexes)
	tails := make([]int, uniqueCount)
	lengths := make([]int, uniqueCount)
	for index := range tails {
		tails[index] = uniqueCount + 1
	}
	for index := range uniqueCount {
		length := sort.Search(uniqueCount, func(candidate int) bool {
			return tails[candidate] >= inverse[index]
		})
		tails[length] = inverse[index]
		lengths[index] = length + 1
	}

	longest := 0
	for _, length := range lengths {
		if longest < length {
			longest = length
		}
	}
	matches := make([]diffPair, 2+longest)
	matches[1+longest] = diffPair{old: len(oldLines), new: len(newLines)}
	lastNewIndex := uniqueCount
	for index := uniqueCount - 1; index >= 0; index-- {
		if lengths[index] == longest && inverse[index] < lastNewIndex {
			matches[longest] = diffPair{old: oldIndexes[index], new: newIndexes[inverse[index]]}
			longest--
		}
	}
	matches[0] = diffPair{}
	return matches
}

// 大量短行会让 anchored diff 的行表、哈希表和匹配数组显著放大。
// 超过工作集预算时使用前后缀算法，只保留常量级状态并输出一个合法 hunk。
func writeLinearTextDiff(writer io.Writer, path, oldContent, newContent string, oldLineCount, newLineCount int) (diffStats, error) {
	oldChangeStart, newChangeStart := 0, 0
	commonPrefixLines := 0
	for oldChangeStart < len(oldContent) && newChangeStart < len(newContent) {
		oldLine, nextOld := nextTextDiffLine(oldContent, oldChangeStart)
		newLine, nextNew := nextTextDiffLine(newContent, newChangeStart)
		if oldLine != newLine {
			break
		}
		oldChangeStart = nextOld
		newChangeStart = nextNew
		commonPrefixLines++
	}

	oldChangeEnd, newChangeEnd := len(oldContent), len(newContent)
	commonSuffixLines := 0
	for oldChangeEnd > oldChangeStart && newChangeEnd > newChangeStart {
		oldLine, previousOld := previousTextDiffLine(oldContent, oldChangeEnd)
		newLine, previousNew := previousTextDiffLine(newContent, newChangeEnd)
		if previousOld < oldChangeStart || previousNew < newChangeStart || oldLine != newLine {
			break
		}
		oldChangeEnd = previousOld
		newChangeEnd = previousNew
		commonSuffixLines++
	}

	prefixContext := commonPrefixLines
	if prefixContext > diffContextLines {
		prefixContext = diffContextLines
	}
	suffixContext := commonSuffixLines
	if suffixContext > diffContextLines {
		suffixContext = diffContextLines
	}

	oldHunkStart := rewindTextDiffLines(oldContent, oldChangeStart, prefixContext)
	oldHunkEnd := advanceTextDiffLines(oldContent, oldChangeEnd, suffixContext)

	oldChangedLines := oldLineCount - commonPrefixLines - commonSuffixLines
	newChangedLines := newLineCount - commonPrefixLines - commonSuffixLines
	oldHunkLineCount := prefixContext + oldChangedLines + suffixContext
	newHunkLineCount := prefixContext + newChangedLines + suffixContext
	stats := diffStats{
		FilesChanged: 1,
		Insertions:   newChangedLines,
		Deletions:    oldChangedLines,
	}

	if err := writeDiffPreamble(writer, path); err != nil {
		return diffStats{}, err
	}
	if err := writeDiffHunkHeader(
		writer,
		unifiedDiffRangeStart(commonPrefixLines-prefixContext, oldHunkLineCount), oldHunkLineCount,
		unifiedDiffRangeStart(commonPrefixLines-prefixContext, newHunkLineCount), newHunkLineCount,
	); err != nil {
		return diffStats{}, err
	}
	if err := writeTextDiffRange(writer, ' ', oldContent, oldHunkStart, oldChangeStart); err != nil {
		return diffStats{}, err
	}
	if err := writeTextDiffRange(writer, '-', oldContent, oldChangeStart, oldChangeEnd); err != nil {
		return diffStats{}, err
	}
	if err := writeTextDiffRange(writer, '+', newContent, newChangeStart, newChangeEnd); err != nil {
		return diffStats{}, err
	}
	if err := writeTextDiffRange(writer, ' ', oldContent, oldChangeEnd, oldHunkEnd); err != nil {
		return diffStats{}, err
	}
	return stats, nil
}

func writeDiffPreamble(writer io.Writer, path string) error {
	for _, value := range []string{
		"diff a/" + path + " b/" + path + "\n",
		"--- a/" + path + "\n",
		"+++ b/" + path + "\n",
	} {
		if err := writeString(writer, value); err != nil {
			return err
		}
	}
	return nil
}

func writeDiffHunkHeader(writer io.Writer, oldStart, oldCount, newStart, newCount int) error {
	return writeString(writer, fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount))
}

func writeDiffLines(writer io.Writer, prefix byte, lines []diffLine) error {
	for _, line := range lines {
		if err := writeDiffLine(writer, prefix, line); err != nil {
			return err
		}
	}
	return nil
}

func writeDiffLine(writer io.Writer, prefix byte, line diffLine) error {
	if err := writeAll(writer, []byte{prefix}); err != nil {
		return err
	}
	if err := writeString(writer, line.text); err != nil {
		return err
	}
	if !line.terminated {
		return writeString(writer, missingDiffLineNewlineMarker)
	}
	return nil
}

func writeTextDiffRange(writer io.Writer, prefix byte, content string, start, end int) error {
	for start < end {
		line, next := nextTextDiffLine(content, start)
		if next > end {
			return fmt.Errorf("diff range ended inside a line")
		}
		if err := writeDiffLine(writer, prefix, line); err != nil {
			return err
		}
		start = next
	}
	return nil
}

func countTextDiffLines(content string) int {
	if content == "" {
		return 0
	}
	count := strings.Count(content, "\n")
	if content[len(content)-1] != '\n' {
		count++
	}
	return count
}

func splitTextDiffLines(content string, lineCount int) []diffLine {
	lines := make([]diffLine, 0, lineCount)
	for offset := 0; offset < len(content); {
		line, next := nextTextDiffLine(content, offset)
		lines = append(lines, line)
		offset = next
	}
	return lines
}

func nextTextDiffLine(content string, start int) (diffLine, int) {
	if newline := strings.IndexByte(content[start:], '\n'); newline >= 0 {
		end := start + newline + 1
		return diffLine{text: content[start:end], terminated: true}, end
	}
	return diffLine{text: content[start:], terminated: false}, len(content)
}

func previousTextDiffLine(content string, end int) (diffLine, int) {
	terminated := content[end-1] == '\n'
	searchEnd := end
	if terminated {
		searchEnd--
	}
	start := strings.LastIndexByte(content[:searchEnd], '\n') + 1
	return diffLine{text: content[start:end], terminated: terminated}, start
}

func rewindTextDiffLines(content string, end, count int) int {
	for range count {
		_, end = previousTextDiffLine(content, end)
	}
	return end
}

func advanceTextDiffLines(content string, start, count int) int {
	for range count {
		_, start = nextTextDiffLine(content, start)
	}
	return start
}

func unifiedDiffRangeStart(zeroBasedStart, count int) int {
	if count == 0 {
		return zeroBasedStart
	}
	return zeroBasedStart + 1
}
