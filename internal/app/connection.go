package app

import (
	"context"
	"fmt"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/matching"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/m3u"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/xmltv"
)

func (s *Service) TestConnection(ctx context.Context, settings config.Settings) error {
	if err := settings.Validate(); err != nil {
		return err
	}

	switch settings.SourceMode {
	case config.SourceModeDirectLogin, config.SourceModeAPIKey:
		return s.dispatcharrFactory(settings).TestConnection(ctx)
	case config.SourceModeXtream:
		client := s.xtreamFactory(settings.XtreamBaseURL, settings.XtreamUsername, settings.XtreamPassword)
		return testXtreamConnection(ctx, client)
	case config.SourceModeM3UXMLTV:
		playlistData, err := s.fetchURL(ctx, settings.M3UURL)
		if err != nil {
			return err
		}
		xmltvData, err := s.fetchURL(ctx, settings.EPGXMLURL)
		if err != nil {
			return err
		}
		entries, err := m3u.Parse(playlistData)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			return fmt.Errorf("no playlist entries available")
		}
		doc, err := xmltv.Parse(xmltvData)
		if err != nil {
			return err
		}
		if len(doc.Programmes) == 0 {
			return fmt.Errorf("epg is required for v1 setup")
		}
		if _, ok := matching.Match(entries[0], doc); !ok {
			return fmt.Errorf("epg does not match playlist entries")
		}
		return nil
	default:
		return fmt.Errorf("source mode %q not implemented", settings.SourceMode)
	}
}

func testXtreamConnection(ctx context.Context, client XtreamClient) error {
	if err := client.TestConnection(ctx); err != nil {
		return err
	}
	streams, err := client.LiveStreams(ctx)
	if err != nil {
		return err
	}
	if len(streams) == 0 {
		return fmt.Errorf("no live streams available")
	}
	epg, err := client.ShortEPG(ctx, streams[0].StreamID)
	if err != nil {
		return err
	}
	if len(epg.EPGListings) == 0 {
		return fmt.Errorf("epg is required for v1 setup")
	}
	return nil
}
