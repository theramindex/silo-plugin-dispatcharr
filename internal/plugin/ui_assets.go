package plugin

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed ui/page.html ui/styles.css ui/app.js
var playerUIAssets embed.FS

var playerPageHTMLTemplate string

func init() {
	playerPageHTMLTemplate = mustLoadPlayerPageHTMLTemplate()
}

func mustLoadPlayerPageHTMLTemplate() string {
	page, err := playerUIAssets.ReadFile("ui/page.html")
	if err != nil {
		panic(fmt.Errorf("read player page template: %w", err))
	}
	styles, err := playerUIAssets.ReadFile("ui/styles.css")
	if err != nil {
		panic(fmt.Errorf("read player styles: %w", err))
	}
	script, err := playerUIAssets.ReadFile("ui/app.js")
	if err != nil {
		panic(fmt.Errorf("read player app script: %w", err))
	}

	body := string(page)
	body = strings.Replace(body, "__APP_STYLES__", string(styles), 1)
	body = strings.Replace(body, "__APP_SCRIPT__", string(script), 1)
	return body
}

func playerAppJavaScript() string {
	script, err := playerUIAssets.ReadFile("ui/app.js")
	if err != nil {
		return ""
	}
	return string(script)
}