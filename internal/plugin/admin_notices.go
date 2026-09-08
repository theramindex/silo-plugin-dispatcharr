package plugin

import (
	_ "embed"
	"html"
)

//go:embed admin/licenses/iptvnator.txt
var guideUpstreamLicense string

func adminLicenseNoticesHTML() string {
	return `<template id="admin-license-notices"><details class="settings-card admin-license-notices"><summary>Open-source notices</summary><p>Guide search, keyboard navigation, and viewport patterns adapted from <a href="https://github.com/4gray/iptvnator/tree/9f950a153042af14caed5c8b596bc3d3d41c418e" target="_blank" rel="noopener noreferrer">IPTVnator</a> under the MIT license.</p><pre>` + html.EscapeString(guideUpstreamLicense) + `</pre></details></template>`
}
