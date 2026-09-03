package plugin

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed ui/page.html ui/styles.css ui/lineup.js ui/sports_replays.js ui/app.js ui/guide.js ui/player.js
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
	return string(page)
}

func playerAppJavaScript() string {
	parts := make([]string, 0, 5)
	for _, name := range []string{"ui/lineup.js", "ui/sports_replays.js", "ui/app.js", "ui/guide.js", "ui/player.js"} {
		payload, err := playerUIAssets.ReadFile(name)
		if err != nil {
			return ""
		}
		parts = append(parts, string(payload))
	}
	return strings.Join(parts, "\n")
}

func playerStylesCSS() string {
	styles, err := playerUIAssets.ReadFile("ui/styles.css")
	if err != nil {
		return ""
	}
	return string(styles)
}
