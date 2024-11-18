package global

type BrowserType string

const (
	Browser_Firefox BrowserType = "firefox"
	Browser_Chrome  BrowserType = "chrome"
)

type Config struct {
	Port int `json:"port" default:"8080" usage:"Port."`
}

var GlobalConfig Config
