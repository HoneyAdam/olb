// olb - OpenLoadBalancer
// A zero-dependency, production-grade load balancer written in Go.
//
// Usage:
//
//	olb start --config /path/to/config.yaml
//	olb stop
//	olb reload
//	olb status
//	olb version
//	olb --help
//
// For more information, visit: https://github.com/openloadbalancer/olb
package main

import (
	"fmt"
	"os"

	"github.com/openloadbalancer/olb/internal/cli"
	"github.com/openloadbalancer/olb/pkg/version"
)

func main() {
	// Create CLI instance
	c := cli.New("olb", version.String())

	// Register all commands
	c.Register(&cli.StartCommand{})
	c.Register(&cli.StopCommand{})
	c.Register(&cli.ReloadCommand{})
	c.Register(&cli.StatusCommand{})
	c.Register(&cli.VersionCommand{})
	c.Register(&cli.ConfigCommand{})
	c.Register(&cli.BackendCommand{})
	c.Register(&cli.HealthCommand{})

	// Run CLI with os.Args
	if err := c.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
