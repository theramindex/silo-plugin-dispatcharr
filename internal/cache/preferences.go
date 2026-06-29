package cache

type PlaybackSettings struct {
	BackendProxyRequested bool   `json:"backendProxyRequested"`
	BackendProxySupported bool   `json:"backendProxySupported"`
	StreamMode            string `json:"streamMode"`
	OutputFormat          string `json:"outputFormat"`
}

type CategoryParsingSettings struct {
	Enabled   bool   `json:"enabled"`
	Mode      string `json:"mode"`
	Delimiter string `json:"delimiter"`
	Regex     string `json:"regex"`
	Output    string `json:"output"`
}

type CustomGroup struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Order int    `json:"order"`
}

type KeywordPass struct {
	ID        string `json:"id"`
	Keyword   string `json:"keyword"`
	CreatedAt int64  `json:"createdAt"`
}

type Preferences struct {
	Favorites              map[string]bool         `json:"favorites"`
	FavoriteOrder          []string                `json:"favoriteOrder"`
	AutoFavorites          map[string]bool         `json:"autoFavorites"`
	HiddenCategories       map[string]bool         `json:"hiddenCategories"`
	SportsFavoriteTeams    map[string]bool         `json:"sportsFavoriteTeams"`
	KeywordPasses          []KeywordPass           `json:"keywordPasses"`
	RecentChannels         []string                `json:"recentChannels"`
	ContinueWatching       map[string]any          `json:"continueWatching"`
	Playback               PlaybackSettings        `json:"playback"`
	CategoryParsing        CategoryParsingSettings `json:"categoryParsing"`
	CustomGroups           []CustomGroup           `json:"customGroups"`
	CustomGroupMemberships map[string][]string     `json:"customGroupMemberships"`
}

func defaultPreferences() Preferences {
	return Preferences{
		Favorites:              map[string]bool{},
		FavoriteOrder:          []string{},
		AutoFavorites:          map[string]bool{},
		HiddenCategories:       map[string]bool{},
		SportsFavoriteTeams:    map[string]bool{},
		KeywordPasses:          []KeywordPass{},
		RecentChannels:         []string{},
		ContinueWatching:       map[string]any{},
		CategoryParsing:        CategoryParsingSettings{Mode: "off", Delimiter: "dash"},
		CustomGroups:           []CustomGroup{},
		CustomGroupMemberships: map[string][]string{},
		Playback: PlaybackSettings{
			BackendProxySupported: false,
			StreamMode:            "redirect",
			OutputFormat:          "ts",
		},
	}
}
