package main

import (
	"os"

	"github.com/robertguss/go-plan/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
