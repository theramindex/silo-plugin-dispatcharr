package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	sharedhttp "github.com/theramindex/silo-plugins/pkg/livemedia/httpclient"
	"github.com/theramindex/silo-plugins/pkg/livemedia/rewind"
	mediaweb "github.com/theramindex/silo-plugins/pkg/livemedia/web"
	"net/http"
	"path/filepath"
	"time"
)

func (s *HTTPRoutesServer) mediaController() *mediaweb.Controller {
	s.mediaOnce.Do(func() {
		s.media = mediaweb.New(mediaweb.Config{Prefix: "/dispatcharr", Root: filepath.Join("/var/lib/continuum/plugins", "silo.ramindex.dispatcharr", "media"), RewindManager: s.timeShift, RewindAllowed: s.timeShiftSourceAllowed, RewindOptions: func() rewind.Config {
			var settings map[string]any
			json.Unmarshal(s.store.AdminSettings(), &settings)
			return mediaweb.RewindSettings(settings)
		}, Resolve: func(ctx context.Context, channel, owner, inputID string) (mediaweb.Input, error) {
			var settingsMap map[string]any
			json.Unmarshal(s.store.AdminSettings(), &settingsMap)
			candidates := s.mediaBackoff.Order(mediaweb.StationOrder(settingsMap["stationMappings"], channel))
			candidate := 0
			var source string
			var err error
			for candidate < len(candidates) {
				source, err = s.resolveStreamURL(candidates[candidate])
				if err == nil {
					break
				}
				candidate++
			}
			if err != nil {
				return mediaweb.Input{}, err
			}
			settings := s.streamProxySettings()
			client := sharedhttp.NewRestricted(configuredStreamHosts(settings)...)
			client.Timeout = 0
			if transport, ok := client.Transport.(*http.Transport); ok {
				transport.ResponseHeaderTimeout = 10 * time.Second
			}
			return mediaweb.Input{URL: source, Container: publicStreamFormat(source), Client: client, Release: nil, Heartbeat: nil, Fallback: func(context.Context) (string, *http.Client, error) {
				s.mediaBackoff.Reject(candidates[candidate])
				for candidate++; candidate < len(candidates); candidate++ {
					next, err := s.resolveStreamURL(candidates[candidate])
					if err == nil && publicStreamFormat(next) == publicStreamFormat(source) {
						return next, client, nil
					}
				}
				return "", nil, fmt.Errorf("no alternate source is available")
			}}, nil
		}})
	})
	return s.media
}
