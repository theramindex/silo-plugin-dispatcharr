package model

type SourceMode string

const (
	SourceModeDirectLogin SourceMode = "direct_login"
	SourceModeAPIKey      SourceMode = "api_key"
	SourceModeXtream      SourceMode = "xtream"
	SourceModeM3UXMLTV    SourceMode = "m3u_xmltv"
	LiveTVSourceID        string     = "source:live-tv"
)

type Source struct {
	ID   string     `json:"id"`
	Name string     `json:"name"`
	Mode SourceMode `json:"mode"`
}

func LiveTVSource(mode SourceMode) Source {
	return Source{ID: LiveTVSourceID, Name: "Live TV", Mode: mode}
}
