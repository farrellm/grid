// Package cli builds the grid command line.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/farrellm/grid/internal/format"
	"github.com/farrellm/grid/internal/source"
	"github.com/farrellm/grid/internal/ui"
)

// version is overridden at build time with -ldflags.
var version = "dev"

type options struct {
	noHeader   bool
	frozenCols int
	bufferSize int
	delimiter  string
	comment    string
	full       bool
	fileFormat string
	follow     bool
}

// dataExts are the file extensions grid knows how to read. Shell completion
// filters the file argument to these; add to the list as formats are added.
var dataExts = []string{"csv", "tsv", "txt", "parquet"}

// Execute runs the command line.
func Execute(ctx context.Context) error {
	var o options
	return fang.Execute(ctx, newCommand(&o), fang.WithVersion(version))
}

// newCommand builds the root command, writing parsed flags into o.
func newCommand(o *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "grid [file]",
		Short: "less for your data",
		Long: "grid browses large tabular datasets in the terminal.\n\n" +
			"It reads CSV (or another delimited format) from a file or standard\n" +
			"input (named either as - or by naming no file at all), or a Parquet\n" +
			"file, and shows it in an interactive grid. Rows load as you scroll,\n" +
			"so a huge file opens immediately, and a live stream displays as it\n" +
			"arrives -- press f to follow it.\n\n" +
			"Press h for help while running, and q to quit.",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// fang.Execute passes the context main supplies down to here, so
			// cancelling it reaches the background readers.
			return run(cmd.Context(), cmd, args, o)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&o.noHeader, "no-header", "n", false, "assume no header row")
	f.IntVarP(&o.frozenCols, "frozen-cols", "f", 1, "freeze NCOLS columns on the left")
	f.IntVarP(&o.bufferSize, "buffer-size", "b", 100, "read NROWS rows to guess column types")
	f.StringVarP(&o.delimiter, "delimiter", "d", "", "use CHAR as the column delimiter (default: sniff)")
	f.StringVarP(&o.comment, "comment", "c", "", "treat lines starting with PREFIX as comments")
	f.BoolVar(&o.full, "full", false, "read all input before displaying, for better column sizing")
	f.StringVar(&o.fileFormat, "format", "", "input format: csv or parquet (default: from the file extension)")
	f.BoolVar(&o.follow, "follow", false, "start following new rows, as f does")

	// The one positional argument is a data file, so offer the files grid can
	// actually read. The candidates are built here rather than delegated to
	// the shell with ShellCompDirectiveFilterFileExt, because fish ignores
	// that directive entirely and bash implements it with _filedir, from the
	// bash-completion package, which is not always installed.
	cmd.ValidArgsFunction = completeDataFile

	// isParquet also takes tsv and text as aliases for csv; only the two
	// documented names are offered.
	registerFlagCompletion(cmd, "format", []string{
		cobra.CompletionWithDesc("csv", "delimited text"),
		cobra.CompletionWithDesc("parquet", "Apache Parquet"),
	})
	registerFlagCompletion(cmd, "delimiter", []string{
		cobra.CompletionWithDesc(",", "comma"),
		cobra.CompletionWithDesc(`\t`, "tab"),
		cobra.CompletionWithDesc(";", "semicolon"),
		cobra.CompletionWithDesc("|", "pipe"),
	})

	return cmd
}

// registerFlagCompletion offers a fixed set of values for a flag. It panics on
// a misspelled flag name, which is a programming error, not a runtime one.
func registerFlagCompletion(cmd *cobra.Command, flag string, choices []string) {
	err := cmd.RegisterFlagCompletionFunc(flag,
		cobra.FixedCompletions(choices, cobra.ShellCompDirectiveNoFileComp))
	if err != nil {
		panic(err)
	}
}

// completeDataFile completes the file argument with directories and the files
// grid can read. Directories keep their trailing separator so completion can
// carry on into them, which is what NoSpace is for.
func completeDataFile(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		// grid takes at most one file.
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	dir, prefix := filepath.Split(toComplete)
	entries, err := os.ReadDir(expandHome(dir))
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var (
		names  []string
		anyDir bool
	)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		// Hidden files stay hidden until they are asked for by name.
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}
		if isDir(filepath.Join(expandHome(dir), name), e) {
			names = append(names, dir+name+string(filepath.Separator))
			anyDir = true
			continue
		}
		if hasDataExt(name) {
			names = append(names, dir+name)
		}
	}

	directive := cobra.ShellCompDirectiveNoFileComp
	if anyDir {
		// A directory is only ever a step on the way to a file.
		directive |= cobra.ShellCompDirectiveNoSpace
	}
	return names, directive
}

// hasDataExt reports whether the name ends in an extension grid can read.
func hasDataExt(name string) bool {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	return slices.Contains(dataExts, ext)
}

// isDir follows symlinks, which ReadDir reports as links rather than as the
// directories they point at.
func isDir(path string, e os.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	if e.Type()&os.ModeSymlink == 0 {
		return false
	}
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// expandHome resolves a leading ~ so completion works inside the home
// directory. An empty path means the working directory.
func expandHome(dir string) string {
	if dir == "" {
		return "."
	}
	if dir == "~/" || strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(dir, "~/"))
		}
	}
	return dir
}

func run(ctx context.Context, cmd *cobra.Command, args []string, o *options) error {
	cfg := format.DefaultConfig()

	var path string
	if len(args) > 0 {
		path = args[0]
	}
	// '-' is the conventional name for standard input, and naming no file at
	// all means the same thing. Settling it here keeps os.Open and the format
	// sniffing from ever seeing a path that is not one.
	fromStdin := path == "" || path == "-"
	if fromStdin {
		path = ""
	}

	if fromStdin && isTerminal(os.Stdin) {
		// Nothing piped in and no file named: there is nothing to show.
		return cmd.Help()
	}

	src, err := openSource(ctx, path, o, cfg)
	if err != nil {
		return err
	}
	defer src.Close()

	// When data arrives on stdin, the keyboard has to come from the terminal
	// itself; ngrid reopened /dev/tty for the same reason.
	var teaOpts []tea.ProgramOption
	if fromStdin {
		tty, err := os.Open("/dev/tty")
		if err != nil {
			return fmt.Errorf("reading from a pipe needs a terminal for input: %w", err)
		}
		defer tty.Close()
		teaOpts = append(teaOpts, tea.WithInput(tty))
	}

	model := ui.New(src, cfg, o.frozenCols, o.follow)
	p := tea.NewProgram(model, teaOpts...)
	if _, err := p.Run(); err != nil {
		return err
	}
	// A read error is reported after the display is torn down, so it is not
	// lost behind the alternate screen.
	return src.Err()
}

// openSource picks a reader for the input and starts it loading.
func openSource(ctx context.Context, path string, o *options, cfg format.Config) (source.Source, error) {
	if isParquet(path, o.fileFormat) {
		if path == "" {
			return nil, errors.New("parquet input must be a file: it cannot be read from a pipe")
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return source.OpenParquet(ctx, f, source.ParquetOptions{
			Filename: path,
			Config:   cfg,
			Full:     o.full,
		})
	}

	var (
		r    io.Reader
		name string
	)
	if path == "" {
		r, name = os.Stdin, "(stdin)"
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		r, name = f, path
	}

	delim, err := parseDelimiter(o.delimiter)
	if err != nil {
		return nil, err
	}

	return source.OpenCSV(ctx, r, source.CSVOptions{
		Filename:      name,
		HasHeader:     !o.noHeader,
		SampleSize:    o.bufferSize,
		Delimiter:     delim,
		CommentPrefix: o.comment,
		Config:        cfg,
		Full:          o.full,
	})
}

func isParquet(path, forced string) bool {
	switch strings.ToLower(forced) {
	case "parquet":
		return true
	case "csv", "tsv", "text":
		return false
	}
	return strings.EqualFold(filepath.Ext(path), ".parquet")
}

// parseDelimiter accepts a single character, or the escapes \t and \0.
func parseDelimiter(s string) (rune, error) {
	switch s {
	case "":
		return 0, nil // sniff
	case `\t`, "\t":
		return '\t', nil
	}
	r := []rune(s)
	if len(r) != 1 {
		return 0, fmt.Errorf("delimiter must be a single character, got %q", s)
	}
	return r[0], nil
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
