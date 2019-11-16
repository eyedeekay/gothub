package main

import (
	"fmt"
	"os"

	"github.com/aktau/github-release/github"
	. "github.com/eyedeekay/gothub"
	"github.com/voxelbrain/goptions"
)

const GH_URL = "https://github.com"

func main() {
	options := Options{}

	goptions.ParseAndFail(&options)

	if options.Version {
		fmt.Printf("gothub v%s\n", VERSION)
		return
	}

	if len(options.Verbs) == 0 {
		goptions.PrintHelp()
		return
	}

	VERBOSITY = len(options.Verbosity)
	github.VERBOSITY = VERBOSITY

	if cmd, found := Commands[options.Verbs]; found {
		err := cmd(options)
		if err != nil {
			if !options.Quiet {
				fmt.Fprintln(os.Stderr, "error:", err)
			}
			os.Exit(1)
		}
	}
}
