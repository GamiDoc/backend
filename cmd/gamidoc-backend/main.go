package main

import (
	"fmt"
	"os"

	"github.com/gamidoc/backend/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
