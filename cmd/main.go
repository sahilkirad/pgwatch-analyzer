package main

import (
	"fmt"
	"os"
	"pgwatchai/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		// Keep error handling centralized at process boundary.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
