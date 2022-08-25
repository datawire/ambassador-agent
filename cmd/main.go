package main

import (
	"fmt"
	"os"

	"github.com/datawire/ambassador-agent/cmd/agent"
)

func main() {
	err := agent.Main()
	if err != nil {
		fmt.Fprintf(os.Stdout, "error executing from argparser: %v", err)
		os.Exit(1)
	}
}
