package main

import (
	"fmt"
	"os"

	"github.com/kosmos-eu/openyard/pkg/command"
	"github.com/kosmos-eu/openyard/pkg/config"
)

func main() {
	cfg := config.Load()
	if err := command.Execute(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "openyard: %v\n", err)
		os.Exit(1)
	}
}
