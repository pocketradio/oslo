package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/pocketradio/oslo/internal/config"
	"github.com/pocketradio/oslo/internal/runtime"
)

func main() {
	// writing logs as json to std o/p
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// example i/p : os.args = ["/usr/local/bin/oslo", "worker"].
	// os.args[1:] becomes ["worker"]. so run receives args[0] as "worker"
	// can also be api/publisher there
	
	if err := run(logger, os.Args[1:]); err != nil {
		logger.Error("oslo stopped", "error", err)
		os.Exit(1)
	}
}

// reads the runtime input supplied after the exec name
// docker and ecs pass api, worker or publisher through os.args[1:]
// the selected i/p determines which process starts

func run(logger *slog.Logger, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: oslo <api|worker|publisher>")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	switch args[0] {
	case "api":
		return runtime.RunAPI(logger, cfg)
	case "worker":
		return runtime.RunWorker(logger, cfg)
	case "publisher":
		return runtime.RunPublisher(logger, cfg)
	default:
		return fmt.Errorf("unknown runtime mode %q", args[0])
	}
}
