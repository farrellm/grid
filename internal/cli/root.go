// Package cli builds the grid command line.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
}

// Execute runs the command line.
func Execute(ctx context.Context) error {
	var o options

	cmd := &cobra.Command{
		Use:   "grid [file]",
		Short: "less for your data",
		Long: "grid browses large tabular datasets in the terminal.\n\n" +
			"It reads CSV (or another delimited format) from a file or standard\n" +
			"input, or a Parquet file, and shows it in an interactive grid.\n" +
			"Rows load as you scroll, so a huge file opens immediately.\n\n" +
			"Press h for help while running, and q to quit.",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, args, &o)
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

	return fang.Execute(ctx, cmd, fang.WithVersion(version))
}

func run(cmd *cobra.Command, args []string, o *options) error {
	cfg := format.DefaultConfig()

	var path string
	if len(args) > 0 {
		path = args[0]
	}

	if path == "" && isTerminal(os.Stdin) {
		// Nothing piped in and no file named: there is nothing to show.
		return cmd.Help()
	}

	src, err := openSource(path, o, cfg)
	if err != nil {
		return err
	}
	defer src.Close()

	// When data arrives on stdin, the keyboard has to come from the terminal
	// itself; ngrid reopened /dev/tty for the same reason.
	var teaOpts []tea.ProgramOption
	if path == "" {
		tty, err := os.Open("/dev/tty")
		if err != nil {
			return fmt.Errorf("reading from a pipe needs a terminal for input: %w", err)
		}
		defer tty.Close()
		teaOpts = append(teaOpts, tea.WithInput(tty))
	}

	model := ui.New(src, cfg, o.frozenCols)
	p := tea.NewProgram(model, teaOpts...)
	if _, err := p.Run(); err != nil {
		return err
	}
	// A read error is reported after the display is torn down, so it is not
	// lost behind the alternate screen.
	return src.Err()
}

// openSource picks a reader for the input and starts it loading.
func openSource(path string, o *options, cfg format.Config) (source.Source, error) {
	if isParquet(path, o.fileFormat) {
		if path == "" {
			return nil, errors.New("parquet input must be a file: it cannot be read from a pipe")
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return source.OpenParquet(f, source.ParquetOptions{
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

	return source.OpenCSV(r, source.CSVOptions{
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
