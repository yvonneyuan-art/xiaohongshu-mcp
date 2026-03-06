package browser

import (
	"encoding/json"
	"net/url"
	"os"
	"runtime"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

type Browser struct {
	browser  *rod.Browser
	launcher *launcher.Launcher
}

type browserConfig struct {
	binPath  string
	headless bool
}

type Option func(*browserConfig)

func WithBinPath(binPath string) Option {
	return func(c *browserConfig) {
		c.binPath = binPath
	}
}

func WithHeadless(headless bool) Option {
	return func(c *browserConfig) {
		c.headless = headless
	}
}

// getDefaultChromePath 获取默认 Chrome 路径
func getDefaultChromePath() string {
	switch runtime.GOOS {
	case "darwin":
		// macOS 默认路径
		return "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	case "linux":
		// Linux 常见路径
		paths := []string{
			"/usr/bin/google-chrome",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
		}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	case "windows":
		// Windows 默认路径
		return "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe"
	}
	return ""
}

// maskProxyCredentials masks username and password in proxy URL for safe logging.
func maskProxyCredentials(proxyURL string) string {
	u, err := url.Parse(proxyURL)
	if err != nil || u.User == nil {
		return proxyURL
	}
	if _, hasPassword := u.User.Password(); hasPassword {
		u.User = url.UserPassword("***", "***")
	} else {
		u.User = url.User("***")
	}
	return u.String()
}

func NewBrowser(headless bool, options ...Option) *Browser {
	cfg := &browserConfig{
		headless: headless,
	}
	for _, opt := range options {
		opt(cfg)
	}

	l := launcher.New().
		Headless(headless).
		Delete("no-startup-window"). // 关键：删除 no-startup-window 以显示窗口
		Set("--no-sandbox").
		Set("--disable-dev-shm-usage").
		Set("--disable-gpu").
		Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	// Set custom Chrome binary path if provided
	if cfg.binPath != "" {
		l = l.Bin(cfg.binPath)
		logrus.Infof("使用指定 Chrome 路径: %s", cfg.binPath)
	} else {
		// 尝试使用默认路径
		if defaultPath := getDefaultChromePath(); defaultPath != "" {
			if _, err := os.Stat(defaultPath); err == nil {
				l = l.Bin(defaultPath)
				logrus.Infof("使用默认 Chrome 路径: %s", defaultPath)
			} else {
				logrus.Warnf("默认 Chrome 路径不存在: %s", defaultPath)
			}
		}
	}

	// Read proxy from environment variable
	if proxy := os.Getenv("XHS_PROXY"); proxy != "" {
		l = l.Proxy(proxy)
		logrus.Infof("Using proxy: %s", maskProxyCredentials(proxy))
	}

	url := l.MustLaunch()
	logrus.Infof("Chrome 启动成功，控制 URL: %s", url)

	browser := rod.New().
		ControlURL(url).
		MustConnect()

	// 加载 cookies
	cookiePath := cookies.GetCookiesFilePath()
	cookieLoader := cookies.NewLoadCookie(cookiePath)
	if data, err := cookieLoader.LoadCookies(); err == nil {
		var cks []*proto.NetworkCookie
		if err := json.Unmarshal(data, &cks); err == nil {
			browser.MustSetCookies(cks...)
			logrus.Debugf("loaded cookies from file successfully")
		}
	} else {
		logrus.Warnf("failed to load cookies: %v", err)
	}

	return &Browser{
		browser:  browser,
		launcher: l,
	}
}

func (b *Browser) Close() {
	b.browser.MustClose()
	b.launcher.Cleanup()
}

func (b *Browser) NewPage() *rod.Page {
	return stealth.MustPage(b.browser)
}

func (b *Browser) GetBrowser() *rod.Browser {
	return b.browser
}
