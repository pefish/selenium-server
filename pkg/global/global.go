package global

type BrowserType string

const (
	Browser_Firefox BrowserType = "firefox"
	Browser_Chrome  BrowserType = "chrome"
)

type Config struct {
	DataDir string `json:"datadir"`
	Port    int    `json:"port"`
}

var GlobalConfig Config
