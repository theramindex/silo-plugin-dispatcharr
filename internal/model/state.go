package model

type SyncHealth struct {
	LastSuccessUnix int64  `json:"lastSuccessUnix"`
	LastFailureUnix int64  `json:"lastFailureUnix"`
	LastError       string `json:"lastError,omitempty"`
}

type CatalogState struct {
	Source   Source       `json:"source"`
	Channels []Channel    `json:"channels"`
	Programs []Program    `json:"programs"`
	Health   SyncHealth   `json:"health"`
	Content  ContentState `json:"content"`
}
