package cache

type PlaybackSettings struct {
	BackendProxyRequested bool   `json:"backendProxyRequested"`
	BackendProxySupported bool   `json:"backendProxySupported"`
	StreamMode            string `json:"streamMode"`
	OutputFormat          string `json:"outputFormat"`
}

type Preferences struct {
	Favorites        map[string]bool  `json:"favorites"`
	AutoFavorites    map[string]bool  `json:"autoFavorites"`
	HiddenCategories map[string]bool  `json:"hiddenCategories"`
	RecentChannels   []string         `json:"recentChannels"`
	ContinueWatching map[string]any   `json:"continueWatching"`
	Playback         PlaybackSettings `json:"playback"`
}

func defaultPreferences() Preferences {
	return Preferences{
		Favorites:        map[string]bool{},
		AutoFavorites:    map[string]bool{},
		HiddenCategories: map[string]bool{},
		RecentChannels:   []string{},
		ContinueWatching: map[string]any{},
		Playback: PlaybackSettings{
			BackendProxySupported: false,
			StreamMode:            "redirect",
			OutputFormat:          "hls",
		},
	}
}
