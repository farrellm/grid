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

## Use

```
grid data.csv           # a delimited file
grid data.parquet       # a Parquet file
psql -c '...' | grid    # anything on standard input
grid --full data.csv    # read it all first, for the best column sizing
```

Rows load as you scroll, so a file of any size — or an endless stream — opens
immediately. The row count in the status bar ends in `+` while more data is
still waiting to be read.

Press `h` for help while running, and `q` to quit.

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
| `P` | First row and column |
| `~` `INSERT` | Toggle the cell cursor |
| `,` `.` | Narrow / widen the column at the cursor |
| `<` `>` | Less / more precision at the cursor |
| `\|` | Cycle the column separator |
| `H` `F` | Toggle the header / footer |
| `/` `?` | Search forward / backward |
| `n` `N` | Repeat the search |
| `c` `C` | Search and scan to the matching column |
| `h` | Help |
| `q` `Q` | Quit |

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

```
go test ./...     # includes ngrid's own formatter suite, ported
go build ./...
```

The formatter tests in `internal/format` are a direct port of ngrid's
`test/formatters.py`, with every expected string carried over unchanged; they
are what pins the rendering to the original.

## License

Copyright (c) 2026, Matthew Farrell. Released under the BSD three-clause
license. See [LICENSE](LICENSE).

ngrid is copyright (c) 2014, Two Sigma Open Source, under the same license.
