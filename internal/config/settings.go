package config

import (
	"fmt"
	"strings"
)

const (
	DefaultChannelRefreshHours = 24
	DefaultEPGRefreshHours     = 24
)

type SourceMode string

const (
	SourceModeDirectLogin SourceMode = "direct_login"
	SourceModeAPIKey      SourceMode = "api_key"
	SourceModeXtream      SourceMode = "xtream"
	SourceModeM3UXMLTV    SourceMode = "m3u_xmltv"
)

type Settings struct {
	SourceMode        SourceMode
	DispatcharrURL    string
	DispatcharrUser   string
	DispatcharrPass   string
	DispatcharrAPIKey string
	XtreamBaseURL     string
	XtreamUsername    string
	XtreamPassword    string
	M3UURL            string
	EPGXMLURL         string
	LiveTVEnabled     bool
	ChannelRefreshH   int
	EPGRefreshH       int
	ModeSwitchWarning string
}

func (s Settings) Validate() error {
	if s.SourceMode == "" && strings.TrimSpace(s.DispatcharrURL) != "" {
		s.SourceMode = SourceModeDirectLogin
	}
	if strings.TrimSpace(s.DispatcharrAPIKey) != "" {
		s.SourceMode = SourceModeAPIKey
	}

	switch s.SourceMode {
	case SourceModeDirectLogin:
		if strings.TrimSpace(s.DispatcharrURL) == "" {
			return fmt.Errorf("dispatcharr url is required")
		}
		if strings.TrimSpace(s.DispatcharrUser) == "" {
			return fmt.Errorf("dispatcharr username is required")
		}
		if strings.TrimSpace(s.DispatcharrPass) == "" {
			return fmt.Errorf("dispatcharr password is required")
		}
	case SourceModeAPIKey:
		if strings.TrimSpace(s.DispatcharrURL) == "" {
			return fmt.Errorf("dispatcharr url is required")
		}
		if strings.TrimSpace(s.DispatcharrAPIKey) == "" {
			return fmt.Errorf("dispatcharr api key is required")
		}
	case SourceModeXtream:
		if strings.TrimSpace(s.XtreamBaseURL) == "" {
			return fmt.Errorf("xtream base url is required")
		}
		if strings.TrimSpace(s.XtreamUsername) == "" {
			return fmt.Errorf("xtream username is required")
		}
		if strings.TrimSpace(s.XtreamPassword) == "" {
			return fmt.Errorf("xtream password is required")
		}
	case SourceModeM3UXMLTV:
		if strings.TrimSpace(s.M3UURL) == "" {
			return fmt.Errorf("m3u url is required")
		}
		if strings.TrimSpace(s.EPGXMLURL) == "" {
			return fmt.Errorf("epg xml url is required")
		}
	default:
		return fmt.Errorf("source mode is required")
	}

	if s.ChannelRefreshH == 0 {
		s.ChannelRefreshH = DefaultChannelRefreshHours
	}
	if s.EPGRefreshH == 0 {
		s.EPGRefreshH = DefaultEPGRefreshHours
	}
	if s.ChannelRefreshH <= 0 {
		return fmt.Errorf("channel refresh interval must be positive")
	}
	if s.EPGRefreshH <= 0 {
		return fmt.Errorf("epg refresh interval must be positive")
	}

	return nil
}
