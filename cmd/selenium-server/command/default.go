package command

import (
	"flag"
	"fmt"
	"github.com/pefish/go-commander"
	go_config "github.com/pefish/go-config"
	"github.com/pefish/selenium-server/pkg/global"
	"github.com/pefish/selenium-server/pkg/util"
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

func (dc *DefaultCommand) DecorateFlagSet(flagSet *flag.FlagSet) error {
	flagSet.String("datadir", "./", "")
	flagSet.Int("port", 8080, "")
	return nil
}

func (dc *DefaultCommand) Init(data *commander.StartData) error {
	err := go_config.ConfigManagerInstance.Unmarshal(&global.GlobalConfig)
	if err != nil {
		return err
	}

	return nil
}

func (dc *DefaultCommand) OnExited(data *commander.StartData) error {
	return nil
}

func (dc *DefaultCommand) Start(data *commander.StartData) error {
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
		return fmt.Errorf("Can not be empty.")
	}

	dataDir := go_config.ConfigManagerInstance.MustGetString("datadir")
	port := go_config.ConfigManagerInstance.MustGetInt("port")
	seleniumPath := path.Join(dataDir, "selenium-server.jar")
	driverPath := path.Join(dataDir, fmt.Sprintf("%sdriver", browserName))

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
			return fmt.Errorf("Can not be empty.")
		}
		err := util.DownloadDeps(data.ExitCancelCtx, browserName, driverBuildVersion)
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
	service, err := selenium.NewSeleniumService(seleniumPath, port, opts...)
	if err != nil {
		return err
	}
	defer service.Stop()
	select {
	case <-data.ExitCancelCtx.Done():
		break
	}
	return nil
}
