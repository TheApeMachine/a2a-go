package main

import (
	"os"

	"github.com/theapemachine/a2a-go/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
