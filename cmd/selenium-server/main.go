package main

import (
	"github.com/pefish/go-commander"
	"github.com/pefish/selenium-server/cmd/selenium-server/command"
	"github.com/pefish/selenium-server/version"
)

func main() {
	commanderInstance := commander.NewCommander(version.AppName, version.Version, version.AppName+" is a template.")
	commanderInstance.RegisterDefaultSubcommand(&commander.SubcommandInfo{
		Desc:       "Use this command by default if you don't set subcommand.",
		Subcommand: command.NewDefaultCommand(),
	})
	err := commanderInstance.Run()
	if err != nil {
		commanderInstance.Logger.Error(err)
	}
}
