# grid

*It's `less` for your data.*

`grid` browses large tabular datasets in the terminal. It is to tables what
[less](https://en.wikipedia.org/wiki/Less_(Unix)) is to text: point it at a CSV
or Parquet file — or pipe data into it — and scroll around.

It is a Go port of [ngrid](https://github.com/twosigma/ngrid), built on
[Bubble Tea](https://github.com/charmbracelet/bubbletea) and backed by
[Apache Arrow](https://github.com/apache/arrow-go). It ships as one static
binary with no Python, NumPy or pandas to install.

## Install

```
go install github.com/farrellm/grid@latest
```

Or from a checkout, which stamps the version from git:

```
make install
```

### Shell completion

`grid` generates its own completion script:

```
grid completion bash > ~/.local/share/bash-completion/completions/grid
grid completion zsh  > "${fpath[1]}/_grid"
grid completion fish > ~/.config/fish/completions/grid.fish
```

Those are the per-user directories, picked up in the next shell with no
further setup. For everyone on the machine, write the bash script to
`/etc/bash_completion.d/grid` instead, as root.

Once it is installed, `grid <TAB>` completes directories and the files grid can
read — `.csv`, `.tsv`, `.txt` and `.parquet` — and `--format` and `--delimiter`
complete their values. grid builds the candidates itself rather than asking the
shell to filter by extension, so bash, zsh and fish all behave the same, and
bash does not need the `bash-completion` package.

## Use

```
grid data.csv           # a delimited file
grid data.parquet       # a Parquet file
psql -c '...' | grid    # anything on standard input
grid -                  # standard input, named explicitly
grid --full data.csv    # read it all first, for the best column sizing
```

Rows load as you scroll, so a file of any size — or an endless stream — opens
immediately. The row count in the status bar ends in `+` while more data is
still waiting to be read.

Press `h` for help while running, and `q` to quit.

### Live streams

A pipe displays as it arrives rather than only once it has produced enough to
fill a buffer, so a slow or endless producer is as usable as a file:

```
tail -f events.csv | grid
kubectl logs -f pod | grid --follow
```

Press `f` to follow: the view pins itself to the last row and keeps up as rows
land, and the status bar reads `FOLLOW`. Press `f` again to stop following and
leave the stream where it is. `--follow` starts that way.

Column types are inferred from a sample of the first rows, and a stream that
cannot supply a full sample quickly is typed from what it has sent so far. A
column that later turns out to be wider than its sample suggested is widened in
place, so a surprising value never breaks the display.

### Flags

| Flag | Meaning |
| --- | --- |
| `-n`, `--no-header` | Assume no header row |
| `-f`, `--frozen-cols N` | Freeze N columns on the left (default 1) |
| `-b`, `--buffer-size N` | Sample N rows to guess column types (default 100) |
| `-d`, `--delimiter CHAR` | Column delimiter (default: sniffed from the data) |
| `-c`, `--comment PREFIX` | Treat lines starting with PREFIX as comments |
| `--full` | Read all input before displaying |
| `--format csv\|parquet` | Override the format implied by the file extension |

### Keys

| | |
| --- | --- |
| `↓` `↑` `RETURN` | Move one line |
| `PGDN` `PGUP` `SPACE` | Move one window |
| `d` `u` | Move half a window |
| `←` `→` | Move one column |
| `g` `HOME` | First row |
| `G` | Last row of the file |
| `END` | Last row read so far |
| `f` | Follow new rows as they arrive |
| `P` | First row and column |
| `~` `INSERT` | Toggle the cell cursor |
| `,` `.` | Narrow / widen the column at the cursor |
| `w` | Toggle widening every column to show its full name |
| `<` `>` | Less / more precision at the cursor |
| `\|` | Cycle the column separator |
| `H` `F` | Toggle the header / footer |
| `/` `?` | Search forward / backward |
| `n` `N` | Repeat the search |
| `c` `C` | Search and scan to the matching column |
| `&` | Filter rows by the column at the cursor |
| `h` | Help |
| `q` `Q` | Quit |

### Filtering

`&` shows only the rows whose value in the column at the cursor passes an
expression, as `&` does for lines in less. It turns the cursor on if it is off.
The expression is a comparison if it starts with an operator (`=`, `==`, `!=`,
`<`, `<=`, `>`, `>=`), and a regular expression otherwise:

```
&^g                  values starting with g
&> 2.5               greater than 2.5
&>= 2024-01-05       on or after a date
&= "a b"             exactly "a b"; quotes keep spaces and force text
&=                   missing values (also &= NA); &!= for present ones
&!^$                 a leading ! negates, as in less
```

Each cell is compared as its own type: numerically in a number column, in time
order in a date column, and as text otherwise. A regular expression matches the
full value, not the possibly elided text on screen. To search for a pattern
that begins with an operator character, escape it: `&\=`.

Filters stack: each `&` narrows the rows further, and the status bar lists
them. An empty `&` clears them all. On a stream, rows are filtered as they
arrive, and more of the input is read until the screen fills.

## Differences from ngrid

- **No pandas.** Data is held as Arrow record batches. `--dataframe` is
  replaced by `--full`, which reads everything before displaying so column
  widths and precision are chosen from every value rather than a sample, and by
  native Parquet input.
- **Search works.** ngrid shipped it commented out and broken; "Fix search"
  headed its todo list. `/`, `?`, `n`, `N`, `c` and `C` all work, matching a
  regular expression against the text as displayed. A search that reaches the
  end of the loaded rows says so and resumes with `n` once more has arrived.
- **Dates and timestamps are recognised** in CSV, not only in Parquet.
- **Blank and `NA` cells are missing values**, not NaN, so one blank no longer
  turns an integer column into floats.
- **Column types adapt.** ngrid guessed from a sample and left widening the
  type as a FIXME; here a value that does not fit widens its column
  (integer → float → text) and the rows are re-read.
- **The help matches the code.** ngrid's help described `,` `.` `<` `>` as
  doing the opposite of what its code did. `,` and `<` decrease, `.` and `>`
  increase.
- **Comment headers do not crash.** ngrid's title-line path called `write()`
  with one argument for a function needing two.
- Column widths are measured in terminal columns, so wide (CJK) text aligns.

## Development

Run `make` for the full list of targets.

```
make build        # build ./grid
make install      # install it, with the version stamped from git
make test         # run the tests
make lint         # run golangci-lint
make check        # what CI runs: format check, vet, lint, race tests
make run          # build, then browse testdata/sample.csv
make fixtures     # regenerate testdata
```

`grid` repaints the whole frame on every keystroke and every batch of rows
that arrives, so rendering is a hot path and is benchmarked. To check a change
against the current code:

```
git stash && make bench-old && git stash pop   # baseline
make bench-new                                 # measure, then benchstat
```

`make bench-new` needs
[benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat).

The formatter tests in `internal/format` are a direct port of ngrid's
`test/formatters.py`, with every expected string carried over unchanged; they
are what pins the rendering to the original.

The files in `testdata` are generated by `testdata/gen.go` from a fixed seed,
so `make fixtures` reproduces them byte for byte on the architecture they were
generated on (amd64). It feeds `math.Exp` into `math.Exp`, which Go does not
promise is bit-identical across architectures, so on arm64 the largest
exponentials differ in their final digits. `widening.csv` is the
awkward one on purpose: a comment header, an integer column that meets a float
400 rows in, another that meets text at 450, and blanks throughout.

## License

Copyright (c) 2026, Matthew Farrell. Released under the BSD three-clause
license. See [LICENSE](LICENSE).

ngrid is copyright (c) 2014, Two Sigma Open Source, under the same license.
