package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// fixtureDir builds a directory holding one of everything completion has to
// decide about, and returns it with a trailing separator, ready to be used as
// a completion prefix.
func fixtureDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	for _, name := range []string{
		"data.csv", "table.tsv", "notes.txt", "events.parquet", // readable
		"README.md", "binary.bin", "Makefile", // not readable
		".hidden.csv", // readable, but hidden
	} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "sub"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	return dir + string(filepath.Separator)
}

func TestCompleteDataFile(t *testing.T) {
	dir := fixtureDir(t)

	tests := []struct {
		name       string
		prefix     string
		want       []string
		notWant    []string
		wantNoSpce bool
	}{
		{
			name:       "offers readable files and directories only",
			prefix:     dir,
			want:       []string{"data.csv", "table.tsv", "notes.txt", "events.parquet", "sub/", "link/"},
			notWant:    []string{"README.md", "binary.bin", "Makefile", ".hidden.csv"},
			wantNoSpce: true,
		},
		{
			name:    "filters on the prefix",
			prefix:  dir + "da",
			want:    []string{"data.csv"},
			notWant: []string{"table.tsv", "sub/"},
		},
		{
			name:   "a hidden file has to be asked for by name",
			prefix: dir + ".",
			want:   []string{".hidden.csv"},
		},
		{
			name:    "an unreadable directory completes nothing",
			prefix:  dir + "nowhere/",
			notWant: []string{"data.csv"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, directive := completeDataFile(nil, nil, tt.prefix)

			// Candidates carry the directory the prefix named, so compare on
			// the part the user actually sees completed.
			var base []string
			for _, g := range got {
				base = append(base, strings.TrimPrefix(g, dir))
			}
			for _, want := range tt.want {
				if !slices.Contains(base, want) {
					t.Errorf("candidates %q missing %q", base, want)
				}
			}
			for _, notWant := range tt.notWant {
				if slices.Contains(base, notWant) {
					t.Errorf("candidates %q should not include %q", base, notWant)
				}
			}
			if directive&cobra.ShellCompDirectiveNoFileComp == 0 {
				t.Error("directive must set NoFileComp, or the shell adds every file back")
			}
			if noSpace := directive&cobra.ShellCompDirectiveNoSpace != 0; noSpace != tt.wantNoSpce {
				t.Errorf("NoSpace = %v, want %v (it lets completion carry on into a directory)", noSpace, tt.wantNoSpce)
			}
		})
	}
}

func TestCompleteDataFileTakesOneFile(t *testing.T) {
	got, directive := completeDataFile(nil, []string{"already.csv"}, "")
	if got != nil {
		t.Errorf("candidates = %q, want none after a file is named", got)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", directive)
	}
}

// complete runs the hidden __complete command the shell scripts call, and
// returns the candidate lines and the directive cobra reported.
func complete(t *testing.T, args ...string) (cands []string, directive string) {
	t.Helper()

	cmd := newCommand(&options{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(append([]string{cobra.ShellCompRequestCmd}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("__complete %v: %v", args, err)
	}

	for _, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "Completion ended with directive: "):
			directive = strings.TrimPrefix(line, "Completion ended with directive: ")
		case strings.HasPrefix(line, ":"):
			// The numeric directive, repeated for the shell.
		default:
			// Descriptions are tab separated from the value.
			cands = append(cands, strings.SplitN(line, "\t", 2)[0])
		}
	}
	return cands, directive
}

func TestFlagCompletion(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "format offers the input formats",
			args: []string{"--format", ""},
			want: []string{"csv", "parquet"},
		},
		{
			name: "delimiter offers common separators",
			args: []string{"--delimiter", ""},
			want: []string{",", `\t`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cands, directive := complete(t, tt.args...)
			if directive != "ShellCompDirectiveNoFileComp" {
				t.Errorf("directive = %s, want ShellCompDirectiveNoFileComp", directive)
			}
			for _, want := range tt.want {
				if !slices.Contains(cands, want) {
					t.Errorf("candidates %q missing %q", cands, want)
				}
			}
		})
	}
}

// TestFlagCompletionIsRegistered catches a misspelled flag name in newCommand
// even if the completion output above were to change shape.
func TestFlagCompletionIsRegistered(t *testing.T) {
	cmd := newCommand(&options{})
	for _, flag := range []string{"format", "delimiter"} {
		if _, ok := cmd.GetFlagCompletionFunc(flag); !ok {
			t.Errorf("no completion function registered for --%s", flag)
		}
	}
}
