package api_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files in testdata")

// refreshHint tells a reader how to rewrite the golden files after an
// intended change. It names the whole package because every golden test
// shares assertMatchesGolden.
const refreshHint = "if the change is intended, run: go test -update . and review the diff"

// assertMatchesGolden compares got with the golden file at path, rewriting
// it first when -update is set.
func assertMatchesGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	got = append(got, '\n')
	if *updateGolden {
		writeGolden(t, path, got)
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (%s)", path, err, refreshHint)
	}
	if diff := firstDifference(want, got); diff != "" {
		t.Fatalf("%s differs from the generated output:\n%s\n%s", path, diff, refreshHint)
	}
}

func writeGolden(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create golden folder: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write golden %s: %v", path, err)
	}
}

// diffContextLines is how many lines before the first difference are shown,
// so a reviewer sees where in the spec the change sits.
const diffContextLines = 3

// firstDifference returns the first line where want and got differ, with a
// few preceding lines of context, or "" when they are equal. A full diff is
// left to git; this only has to point the reader at the change.
func firstDifference(want, got []byte) string {
	if bytes.Equal(want, got) {
		return ""
	}
	wantLines := strings.Split(string(want), "\n")
	gotLines := strings.Split(string(got), "\n")
	index := 0
	for index < len(wantLines) && index < len(gotLines) && wantLines[index] == gotLines[index] {
		index++
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "first difference at line %d\n", index+1)
	for context := max(0, index-diffContextLines); context < index; context++ {
		fmt.Fprintf(&builder, "  %s\n", wantLines[context])
	}
	fmt.Fprintf(&builder, "- want: %s\n", lineOrAbsent(wantLines, index))
	fmt.Fprintf(&builder, "+ got:  %s\n", lineOrAbsent(gotLines, index))
	return builder.String()
}

func lineOrAbsent(lines []string, index int) string {
	if index >= len(lines) {
		return "<absent>"
	}
	return lines[index]
}

func TestFirstDifference(t *testing.T) {
	tests := []struct {
		name        string
		want        string
		got         string
		contains    []string
		notContains []string
	}{
		{name: "equal", want: "a\nb\n", got: "a\nb\n"},
		{
			name:        "differing line",
			want:        "a\nb\nc\nd\ne\n",
			got:         "a\nb\nc\nd\nX\n",
			contains:    []string{"line 5", "- want: e\n", "+ got:  X\n", "  b\n", "  d\n"},
			notContains: []string{"  a\n"},
		},
		{
			name:     "got is longer",
			want:     "a",
			got:      "a\nb",
			contains: []string{"line 2", "- want: <absent>", "+ got:  b\n"},
		},
		{
			name:     "want is longer",
			want:     "a\nb",
			got:      "a",
			contains: []string{"line 2", "- want: b\n", "+ got:  <absent>"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diff := firstDifference([]byte(test.want), []byte(test.got))

			if len(test.contains) == 0 && diff != "" {
				t.Fatalf("firstDifference() = %q, want empty", diff)
			}
			for _, fragment := range test.contains {
				if !strings.Contains(diff, fragment) {
					t.Errorf("firstDifference() = %q, want it to contain %q", diff, fragment)
				}
			}
			for _, fragment := range test.notContains {
				if strings.Contains(diff, fragment) {
					t.Errorf("firstDifference() = %q, want it not to contain %q", diff, fragment)
				}
			}
		})
	}
}
