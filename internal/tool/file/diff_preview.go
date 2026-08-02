package file

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	maxDiffOutputBytes         = 64 << 20
	maxBufferedDiffHeaderBytes = 64 << 10
	initialDiffPreviewCapacity = 64 << 10
)

type diffStats struct {
	FilesChanged int
	Insertions   int
	Deletions    int
}

type textDiffInput struct {
	Path             string
	OldContent       string
	NewContent       string
	ForceFileChanged bool
}

func unifiedDiffPreview(path, oldContent, newContent string, maxBytes int) (string, bool, diffStats, error) {
	return unifiedDiffPreviewFiles([]textDiffInput{{
		Path:       path,
		OldContent: oldContent,
		NewContent: newContent,
	}}, maxBytes)
}

func unifiedDiffPreviewFiles(inputs []textDiffInput, maxBytes int) (string, bool, diffStats, error) {
	if maxBytes <= 0 {
		maxBytes = maxDiffOutputBytes
	}
	output := newBoundedDiffWriter(maxBytes, maxDiffOutputBytes)
	total := diffStats{}
	for _, input := range inputs {
		if input.OldContent == input.NewContent {
			if input.ForceFileChanged {
				total.FilesChanged++
			}
			continue
		}

		unifiedOutput := newUnifiedDiffWriter(output)
		stats, err := writeTextDiff(unifiedOutput, input.Path, input.OldContent, input.NewContent)
		if err == nil {
			err = unifiedOutput.Close()
		}
		if err != nil {
			return "", false, diffStats{}, err
		}
		if stats.FilesChanged > 0 {
			total.FilesChanged++
		}
		total.Insertions += stats.Insertions
		total.Deletions += stats.Deletions
	}
	preview, truncated := output.Result()
	return preview, truncated, total, nil
}

type diffOutputLimitError struct {
	limit    int
	observed int
}

func (err *diffOutputLimitError) Error() string {
	return fmt.Sprintf("diff output exceeds %d bytes (observed at least %d bytes)", err.limit, err.observed)
}

// boundedDiffWriter 只保留预览所需的前 maxPreview 字节，同时持续统计完整输出大小。
// 达到全局上限时立即返回错误，避免先在主进程中构造完整 diff。
type boundedDiffWriter struct {
	preview    []byte
	maxPreview int
	limit      int
	total      int
}

func newBoundedDiffWriter(maxPreview, limit int) *boundedDiffWriter {
	if maxPreview < 0 {
		maxPreview = 0
	}
	if maxPreview > limit {
		maxPreview = limit
	}
	initialCapacity := maxPreview
	if initialCapacity > initialDiffPreviewCapacity {
		initialCapacity = initialDiffPreviewCapacity
	}
	return &boundedDiffWriter{
		preview:    make([]byte, 0, initialCapacity),
		maxPreview: maxPreview,
		limit:      limit,
	}
}

func (writer *boundedDiffWriter) Write(data []byte) (int, error) {
	if len(data) > writer.limit-writer.total {
		return 0, &diffOutputLimitError{limit: writer.limit, observed: writer.total + len(data)}
	}
	writer.total += len(data)
	if remaining := writer.maxPreview - len(writer.preview); remaining > 0 {
		if remaining > len(data) {
			remaining = len(data)
		}
		writer.growPreview(remaining)
		writer.preview = append(writer.preview, data[:remaining]...)
	}
	return len(data), nil
}

func (writer *boundedDiffWriter) WriteString(value string) (int, error) {
	if len(value) > writer.limit-writer.total {
		return 0, &diffOutputLimitError{limit: writer.limit, observed: writer.total + len(value)}
	}
	writer.total += len(value)
	if remaining := writer.maxPreview - len(writer.preview); remaining > 0 {
		if remaining > len(value) {
			remaining = len(value)
		}
		writer.growPreview(remaining)
		writer.preview = append(writer.preview, value[:remaining]...)
	}
	return len(value), nil
}

func (writer *boundedDiffWriter) growPreview(additional int) {
	required := len(writer.preview) + additional
	if required <= cap(writer.preview) {
		return
	}
	capacity := cap(writer.preview) * 2
	if capacity < required {
		capacity = required
	}
	if capacity > writer.maxPreview {
		capacity = writer.maxPreview
	}
	preview := make([]byte, len(writer.preview), capacity)
	copy(preview, writer.preview)
	writer.preview = preview
}

func (writer *boundedDiffWriter) Result() (string, bool) {
	cut := len(writer.preview)
	if writer.total > cut {
		// 输入文本要求 UTF-8，但截断位置仍可能落在一个多字节字符中。
		for cut > 0 && !utf8.Valid(writer.preview[:cut]) {
			cut--
		}
	}
	return string(writer.preview[:cut]), writer.total > cut
}

// unifiedDiffWriter 兼容 anchored diff 的命令头，但只在第一行确实以 "diff "
// 开头时删除它。这样即使底层实现未来直接输出 unified diff，也不会误删 "---"。
type unifiedDiffWriter struct {
	destination io.Writer
	firstLine   []byte
	decided     bool
}

func newUnifiedDiffWriter(destination io.Writer) *unifiedDiffWriter {
	return &unifiedDiffWriter{destination: destination}
}

func (writer *unifiedDiffWriter) Write(data []byte) (int, error) {
	if writer.decided {
		return writer.destination.Write(data)
	}

	newline := bytes.IndexByte(data, '\n')
	if newline < 0 {
		if len(writer.firstLine)+len(data) > maxBufferedDiffHeaderBytes {
			return 0, fmt.Errorf("diff first line exceeds %d bytes", maxBufferedDiffHeaderBytes)
		}
		writer.firstLine = append(writer.firstLine, data...)
		return len(data), nil
	}

	if len(writer.firstLine)+newline+1 > maxBufferedDiffHeaderBytes {
		return 0, fmt.Errorf("diff first line exceeds %d bytes", maxBufferedDiffHeaderBytes)
	}
	writer.firstLine = append(writer.firstLine, data[:newline+1]...)
	if !bytes.HasPrefix(writer.firstLine, []byte("diff ")) {
		if err := writeAll(writer.destination, writer.firstLine); err != nil {
			return 0, err
		}
	}
	writer.firstLine = nil
	writer.decided = true
	if err := writeAll(writer.destination, data[newline+1:]); err != nil {
		return 0, err
	}
	return len(data), nil
}

func (writer *unifiedDiffWriter) WriteString(value string) (int, error) {
	if writer.decided {
		if err := writeString(writer.destination, value); err != nil {
			return 0, err
		}
		return len(value), nil
	}

	newline := strings.IndexByte(value, '\n')
	if newline < 0 {
		if len(writer.firstLine)+len(value) > maxBufferedDiffHeaderBytes {
			return 0, fmt.Errorf("diff first line exceeds %d bytes", maxBufferedDiffHeaderBytes)
		}
		writer.firstLine = append(writer.firstLine, value...)
		return len(value), nil
	}

	if len(writer.firstLine)+newline+1 > maxBufferedDiffHeaderBytes {
		return 0, fmt.Errorf("diff first line exceeds %d bytes", maxBufferedDiffHeaderBytes)
	}
	writer.firstLine = append(writer.firstLine, value[:newline+1]...)
	if !bytes.HasPrefix(writer.firstLine, []byte("diff ")) {
		if err := writeAll(writer.destination, writer.firstLine); err != nil {
			return 0, err
		}
	}
	writer.firstLine = nil
	writer.decided = true
	if err := writeString(writer.destination, value[newline+1:]); err != nil {
		return 0, err
	}
	return len(value), nil
}

func (writer *unifiedDiffWriter) Close() error {
	if writer.decided || len(writer.firstLine) == 0 {
		return nil
	}
	writer.decided = true
	return writeAll(writer.destination, writer.firstLine)
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}

func writeString(writer io.Writer, value string) error {
	for len(value) > 0 {
		if stringWriter, ok := writer.(io.StringWriter); ok {
			written, err := stringWriter.WriteString(value)
			if err != nil {
				return err
			}
			if written == 0 {
				return io.ErrShortWrite
			}
			value = value[written:]
			continue
		}

		chunkSize := len(value)
		if chunkSize > 32<<10 {
			chunkSize = 32 << 10
		}
		written, err := writer.Write([]byte(value[:chunkSize]))
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		value = value[written:]
	}
	return nil
}

func countDiffStats(diffText string) diffStats {
	stats := diffStats{}
	if strings.TrimSpace(diffText) == "" {
		return stats
	}
	for _, line := range strings.Split(diffText, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- "):
			continue
		case strings.HasPrefix(line, "+"):
			stats.Insertions++
		case strings.HasPrefix(line, "-"):
			stats.Deletions++
		}
	}
	if stats.Insertions > 0 || stats.Deletions > 0 {
		stats.FilesChanged = 1
	}
	return stats
}
