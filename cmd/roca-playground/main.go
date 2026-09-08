package main

import (
	"fmt"
	"github.com/thellmwhisperer/la-roca/plugins/playground/internal/cli"
	"os"
)

func main() {
	code, err := cli.Execute(cli.Build{Version: "0.1.1"}, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
