package main

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/network-support/server"
	"github.com/urfave/cli"
)

// VERSION of the binary, that can be changed during build
var VERSION = "v0.0.0-dev"

func main() {
	app := cli.NewApp()
	app.Name = "network-support"
	app.Version = VERSION
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Turn on debug logging",
		},
	}

	app.Action = run
	app.Run(os.Args)
}

func run(c *cli.Context) error {
	if c.Bool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}
	s, err := server.NewServer()
	if err != nil {
		logrus.Errorf("Error creating new server: %v", err)
		return err
	}

	if err := s.Run(); err != nil {
		logrus.Errorf("Failed to start: %v", err)
	}

	<-s.GetExitChannel()
	logrus.Infof("Program exiting")
	return nil
}
