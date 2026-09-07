// Command grid browses large tabular datasets in the terminal.
//
// It is a Go port of ngrid (https://github.com/twosigma/ngrid), built on Bubble
// Tea and backed by Apache Arrow.
package main

import (
	"context"
	"os"

	"github.com/farrellm/grid/internal/cli"
)

func main() {
	if err := cli.Execute(context.Background()); err != nil {
		// fang has already reported the error.
		os.Exit(1)
	}
}
