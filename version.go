package main

import (
	"context"
	"fmt"
	"runtime"

	"github.com/urfave/cli/v3"
)

var (
	Version     = "dev"
	GitCommit   = "dev"
	BuildDate   = "unknown"
	VersionDate = "unknown"
)

func getVersionCommand() *cli.Command {
	return &cli.Command{
		Name:    "version",
		Aliases: []string{"v"},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name: "full",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.Bool("full") {
				fmt.Println("Version:          ", Version)
				fmt.Println("GitCommit:        ", GitCommit)
				fmt.Println("BuildDate:        ", BuildDate)
				fmt.Println("OS/Arch:          ", runtime.GOOS, "/", runtime.GOARCH)
			} else {
				fmt.Println(Version)
			}
			return nil
		},
	}
}
