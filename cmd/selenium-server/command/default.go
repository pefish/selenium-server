package command

import (
	"fmt"

	"github.com/pefish/go-commander"
	go_config "github.com/pefish/go-config"
	"github.com/pefish/selenium-server/pkg/global"
	"github.com/pefish/selenium-server/pkg/util"
	"github.com/pkg/errors"
	"github.com/tarantool/go-prompt"
	"github.com/tebeka/selenium"

	"os"
	"path"
)

type DefaultCommand struct {
}

func NewDefaultCommand() *DefaultCommand {
	return &DefaultCommand{}
}

func (dc *DefaultCommand) Config() interface{} {
	return &global.GlobalConfig
}

func (dc *DefaultCommand) Data() interface{} {
	return nil
}

func (dc *DefaultCommand) Init(commander *commander.Commander) error {
	err := go_config.ConfigManagerInstance.Unmarshal(&global.GlobalConfig)
	if err != nil {
		return err
	}

	return nil
}

func (dc *DefaultCommand) OnExited(commander *commander.Commander) error {
	return nil
}

func (dc *DefaultCommand) Start(commander *commander.Commander) error {
	fmt.Println("Please select browser.")
	browserName := prompt.New(
		func(s string) {},
		func(d prompt.Document) []prompt.Suggest {
			return prompt.FilterHasPrefix(
				[]prompt.Suggest{
					{
						Text: "chrome",
					},
					{
						Text: "firefox",
					},
				},
				d.GetWordBeforeCursor(),
				true,
			)
		}, prompt.OptionPrefix(">>> ")).
		Input()
	if browserName == "" {
		return errors.New("Browser name can not be empty.")
	}

	seleniumPath := path.Join(commander.DataDir, "selenium-server.jar")
	driverPath := path.Join(commander.DataDir, fmt.Sprintf("%sdriver", browserName))

	if !util.FileExists(seleniumPath) || !util.FileExists(driverPath) {
		fmt.Println("Please input driver build version.")
		driverBuildVersion := prompt.New(
			func(s string) {},
			func(d prompt.Document) []prompt.Suggest {
				switch browserName {
				case string(global.Browser_Chrome):
					return prompt.FilterHasPrefix(
						[]prompt.Suggest{
							{
								Text:        "1145904",
								Description: "浏览器版本应当高个 1，比如这里是 114，浏览器应该是 115",
							},
						},
						d.GetWordBeforeCursor(),
						true,
					)
				case string(global.Browser_Firefox):
					return prompt.FilterHasPrefix(
						[]prompt.Suggest{
							{
								Text: "68.0.1",
							},
						},
						d.GetWordBeforeCursor(),
						true,
					)
				default:
					return nil
				}
			}, prompt.OptionPrefix(">>> ")).
			Input()

		if driverBuildVersion == "" {
			commander.Logger.Info("没有选择版本，即将下载最新版本")
		}
		err := util.DownloadDeps(commander.Ctx, commander.Logger, browserName, driverBuildVersion)
		if err != nil {
			return err
		}
	}

	opts := []selenium.ServiceOption{}
	if browserName == "firefox" {
		driverPath = "geckodriver"
		opts = []selenium.ServiceOption{
			//selenium.StartFrameBuffer(),
			selenium.GeckoDriver(driverPath),
			selenium.Output(os.Stderr),
		}
	} else if browserName == "chrome" {
		driverPath = "chromedriver"
		opts = []selenium.ServiceOption{
			//selenium.StartFrameBuffer(),
			selenium.ChromeDriver(driverPath),
			selenium.Output(os.Stderr),
		}
	} else {
		return fmt.Errorf("browser config error")
	}
	selenium.SetDebug(true)
	service, err := selenium.NewSeleniumService(seleniumPath, global.GlobalConfig.Port, opts...)
	if err != nil {
		return err
	}
	defer service.Stop()
	select {
	case <-commander.Ctx.Done():
		break
	}
	return nil
}
