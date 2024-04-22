package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mitchelldw01/omnirepo/internal/log"
	"github.com/mitchelldw01/omnirepo/run"
)

var version = "dev"

func main() {
	args, opts, err := parseRawArguments(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	if opts.Help {
		printHelpMenu()
		return
	}

	if opts.Version {
		fmt.Println(version)
		return
	}

	if err := processCommand(args, opts); err != nil {
		log.Fatal(err)
	}
}

func parseRawArguments(args []string) ([]string, run.Options, error) {
	fs := flag.NewFlagSet("omni", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	opts := run.Options{}

	fs.BoolVar(&opts.Help, "help", false, "")
	fs.BoolVar(&opts.Help, "h", false, "")
	fs.BoolVar(&opts.NoCache, "no-cache", false, "")
	fs.BoolVar(&opts.NoColor, "no-color", false, "")
	fs.BoolVar(&opts.Remote, "remote", false, "")
	fs.BoolVar(&opts.Remote, "r", false, "")
	fs.StringVar(&opts.Target, "target", "", "")
	fs.StringVar(&opts.Target, "t", "", "")
	fs.BoolVar(&opts.Version, "version", false, "")
	fs.BoolVar(&opts.Version, "v", false, "")

	err := fs.Parse(args)
	return fs.Args(), opts, err
}

func printHelpMenu() {
	code := log.Bold + log.Underline
	text := "High performance task-runner for any codebase\n\n"
	text += fmt.Sprintf("%sUsage:%s\n    omni [OPTIONS] [COMMAND] [TASKS...]\n\n", code, log.Reset)

	text += fmt.Sprintf("%sCommands:%s\n", code, log.Reset)
	text += "    unlock                             Forcefully unlock the cache\n"
	text += "    graph                              Show the dependency graph as JSON\n"
	text += "    run                                Run tasks (default)\n\n"

	text += fmt.Sprintf("%sOptions:%s\n", code, log.Reset)
	text += "    -h, --help                         Show help\n"
	text += "    --no-cache                         Invalidate the cache before running task\n"
	text += "    --no-color                         Disable color output\n"
	text += "    -r, --remote                       Use remote cache\n"
	text += "    -t, --target <PATH>                Load tasks from specific target directory\n"

	fmt.Print(text)
}

func processCommand(args []string, opts run.Options) error {
	if opts.NoColor {
		log.NoColor = true
	}

	cmd, tasks, err := parsePositionalArguments(args)
	if err != nil {
		log.Fatal(err)
	}

	if err := validateTaskNames(tasks); err != nil {
		log.Fatal(err)
	}

	return run.RunCommand(cmd, tasks, opts)
}

func parsePositionalArguments(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, errors.New("missing required argument for task(s)")
	}

	switch args[0] {
	case "unlock":
		return "unlock", nil, nil
	case "tree":
		return "tree", args[1:], nil
	case "run":
		return "run", args[1:], nil
	default:
		return "run", args, nil
	}
}

func validateTaskNames(tasks []string) error {
	for _, t := range tasks {
		if strings.Contains(t, ":") {
			return fmt.Errorf("task name '%s' cannot contain ':'", t)
		}
	}

	return nil
}
