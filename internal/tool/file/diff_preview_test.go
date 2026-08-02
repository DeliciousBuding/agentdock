package file

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestUnifiedDiffPreviewWithoutExternalDiff(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	preview, truncated, stats, err := unifiedDiffPreview(
		"example.txt",
		"alpha\nkeep\n",
		"beta\nkeep\n",
		65536,
	)
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Fatal("small diff was truncated")
	}
	for _, want := range []string{
		"--- a/example.txt\n",
		"+++ b/example.txt\n",
		"-alpha\n",
		"+beta\n",
	} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview does not contain %q:\n%s", want, preview)
		}
	}
	if strings.HasPrefix(preview, "diff ") {
		t.Fatalf("preview contains an unexpected command header:\n%s", preview)
	}
	if stats != (diffStats{FilesChanged: 1, Insertions: 1, Deletions: 1}) {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestUnifiedDiffPreviewPreservesMissingNewlineMarkers(t *testing.T) {
	preview, _, _, err := unifiedDiffPreview("example.txt", "alpha", "beta", 65536)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(preview, "\\ No newline at end of file"); got != 2 {
		t.Fatalf("missing newline markers = %d, want 2:\n%s", got, preview)
	}
}

func TestUnifiedDiffPreviewUsesEmptyRanges(t *testing.T) {
	tests := []struct {
		name string
		old  string
		new  string
		want string
	}{
		{name: "add", new: "alpha\n", want: "@@ -0,0 +1,1 @@"},
		{name: "delete", old: "alpha\n", want: "@@ -1,1 +0,0 @@"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			preview, _, _, err := unifiedDiffPreview("example.txt", test.old, test.new, 65536)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(preview, test.want) {
				t.Fatalf("preview does not contain %q:\n%s", test.want, preview)
			}
		})
	}
}

func TestUnifiedDiffPreviewTruncatesAfterCollectingStats(t *testing.T) {
	preview, truncated, stats, err := unifiedDiffPreview("example.txt", "alpha\n", "beta\n", 16)
	if err != nil {
		t.Fatal(err)
	}
	if !truncated || len([]byte(preview)) > 16 {
		t.Fatalf("preview length = %d, truncated = %v", len([]byte(preview)), truncated)
	}
	if stats != (diffStats{FilesChanged: 1, Insertions: 1, Deletions: 1}) {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestUnifiedDiffPreviewReturnsEmptyForIdenticalContent(t *testing.T) {
	preview, truncated, stats, err := unifiedDiffPreview("example.txt", "same\n", "same\n", 65536)
	if err != nil {
		t.Fatal(err)
	}
	if preview != "" || truncated || stats != (diffStats{}) {
		t.Fatalf("preview = %q, truncated = %v, stats = %#v", preview, truncated, stats)
	}
}

func TestUnifiedDiffPreviewProducesSeparateHunks(t *testing.T) {
	oldContent := "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\neleven\ntwelve\n"
	newContent := "one\nTWO\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\nELEVEN\ntwelve\n"

	preview, truncated, stats, err := unifiedDiffPreview("example.txt", oldContent, newContent, 65536)
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Fatal("small diff was truncated")
	}
	if got := strings.Count(preview, "@@ "); got != 2 {
		t.Fatalf("hunk count = %d, want 2:\n%s", got, preview)
	}
	if stats != (diffStats{FilesChanged: 1, Insertions: 2, Deletions: 2}) {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestUnifiedDiffPreviewRejectsOutputOverGlobalLimit(t *testing.T) {
	oldContent := strings.Repeat("a", maxTextFileReadBytes)
	newContent := strings.Repeat("b", maxTextFileReadBytes)

	_, _, _, err := unifiedDiffPreview("large.txt", oldContent, newContent, 64)
	var limitError *diffOutputLimitError
	if !errors.As(err, &limitError) {
		t.Fatalf("error = %v, want diffOutputLimitError", err)
	}
	if limitError.limit != maxDiffOutputBytes || limitError.observed <= maxDiffOutputBytes {
		t.Fatalf("limit error = %#v", limitError)
	}
}

func TestUnifiedDiffPreviewUsesBoundedLinearFallbackForManyShortLines(t *testing.T) {
	lineCount := maxAnchoredDiffWorkingBytes/estimatedAnchoredBytesPerLine/2 + 1
	if selectDiffAlgorithm(lineCount, lineCount) != diffAlgorithmLinear {
		t.Fatal("many short lines did not select the bounded fallback")
	}

	oldContent := strings.Repeat("same\n", lineCount)
	changeOffset := len(oldContent) / 2
	changeOffset -= changeOffset % len("same\n")
	newContent := oldContent[:changeOffset] + "news\n" + oldContent[changeOffset+len("same\n"):]

	preview, truncated, stats, err := unifiedDiffPreview("many-lines.txt", oldContent, newContent, 65536)
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Fatal("single-line fallback diff was truncated")
	}
	if !strings.Contains(preview, "-same\n+news\n") {
		t.Fatalf("fallback preview does not contain the changed line:\n%s", preview)
	}
	if stats != (diffStats{FilesChanged: 1, Insertions: 1, Deletions: 1}) {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestBoundedDiffWriterStoresOnlyPreviewLimit(t *testing.T) {
	writer := newBoundedDiffWriter(32, 1024)
	payload := strings.Repeat("x", 512)
	if _, err := writer.WriteString(payload); err != nil {
		t.Fatal(err)
	}
	preview, truncated := writer.Result()
	if len(writer.preview) != 32 || cap(writer.preview) != 32 {
		t.Fatalf("preview storage len/cap = %d/%d, want 32/32", len(writer.preview), cap(writer.preview))
	}
	if len(preview) != 32 || !truncated || writer.total != len(payload) {
		t.Fatalf("preview len = %d, truncated = %v, total = %d", len(preview), truncated, writer.total)
	}

	lazyWriter := newBoundedDiffWriter(maxTextOutputBytes, maxDiffOutputBytes)
	if cap(lazyWriter.preview) != initialDiffPreviewCapacity {
		t.Fatalf("initial preview capacity = %d, want %d", cap(lazyWriter.preview), initialDiffPreviewCapacity)
	}
}

func TestUnifiedDiffPreviewFilesSharesOneBoundedPreview(t *testing.T) {
	preview, truncated, stats, err := unifiedDiffPreviewFiles([]textDiffInput{
		{Path: "first.txt", OldContent: "old-first\n", NewContent: "new-first\n"},
		{Path: "second.txt", OldContent: "old-second\n", NewContent: "new-second\n"},
	}, 48)
	if err != nil {
		t.Fatal(err)
	}
	if !truncated || len([]byte(preview)) > 48 {
		t.Fatalf("preview length = %d, truncated = %v", len([]byte(preview)), truncated)
	}
	if stats != (diffStats{FilesChanged: 2, Insertions: 2, Deletions: 2}) {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestUnifiedDiffPreviewFilesCountsEmptyNewFile(t *testing.T) {
	preview, truncated, stats, err := unifiedDiffPreviewFiles([]textDiffInput{
		{Path: "empty.txt", ForceFileChanged: true},
	}, 65536)
	if err != nil {
		t.Fatal(err)
	}
	if preview != "" || truncated {
		t.Fatalf("preview = %q, truncated = %v", preview, truncated)
	}
	if stats != (diffStats{FilesChanged: 1}) {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestUnifiedDiffWriterOnlyRemovesDiffCommandHeader(t *testing.T) {
	tests := []struct {
		name   string
		chunks []string
		want   string
	}{
		{
			name:   "command header split across writes",
			chunks: []string{"dif", "f a/file b/file\n--- a/file\n", "+++ b/file\n"},
			want:   "--- a/file\n+++ b/file\n",
		},
		{
			name:   "unified header is preserved",
			chunks: []string{"--- a/file\n", "+++ b/file\n"},
			want:   "--- a/file\n+++ b/file\n",
		},
		{
			name:   "similar prefix is preserved",
			chunks: []string{"diffusion is not a command\n", "--- a/file\n"},
			want:   "diffusion is not a command\n--- a/file\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var destination bytes.Buffer
			writer := newUnifiedDiffWriter(&destination)
			for _, chunk := range test.chunks {
				if _, err := writer.WriteString(chunk); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if destination.String() != test.want {
				t.Fatalf("output = %q, want %q", destination.String(), test.want)
			}
		})
	}
}

func TestUnifiedDiffWriterChecksHeaderAcrossByteWrites(t *testing.T) {
	var destination bytes.Buffer
	writer := newUnifiedDiffWriter(&destination)
	for _, chunk := range [][]byte{
		[]byte("dif"),
		[]byte("f a/file b/file\n--- a/file\n"),
		[]byte("+++ b/file\n"),
	} {
		if _, err := writer.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if got, want := destination.String(), "--- a/file\n+++ b/file\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
