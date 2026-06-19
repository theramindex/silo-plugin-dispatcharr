package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/mapping"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/matching"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/m3u"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/xmltv"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/xtream"
)

type xtreamAppCatalogClient interface {
	LiveCategories(ctx context.Context) ([]xtream.LiveCategory, error)
	VODCategories(ctx context.Context) ([]xtream.VODCategory, error)
	VODStreams(ctx context.Context) ([]xtream.VODStream, error)
	SeriesCategories(ctx context.Context) ([]xtream.SeriesCategory, error)
	Series(ctx context.Context) ([]xtream.Series, error)
}

func (s *Service) SyncNow(ctx context.Context, settings config.Settings, nowUnix int64) error {
	if err := settings.Validate(); err != nil {
		return err
	}

	switch settings.SourceMode {
	case config.SourceModeDirectLogin, config.SourceModeAPIKey:
		if err := s.syncDispatcharr(ctx, settings, nowUnix); err != nil {
			if settings.SourceMode == config.SourceModeDirectLogin {
				if fallbackErr := s.syncXtream(ctx, settings.DispatcharrURL, settings.DispatcharrUser, settings.DispatcharrPass, model.SourceModeDirectLogin, nowUnix); fallbackErr == nil {
					s.StartAsyncEPGRefresh(settings)
					return nil
				} else {
					return fmt.Errorf("dispatcharr REST sync failed (%v); xtream fallback failed: %w", err, fallbackErr)
				}
			}
			return err
		}
		return nil
	case config.SourceModeXtream:
		return s.syncXtream(ctx, settings.XtreamBaseURL, settings.XtreamUsername, settings.XtreamPassword, model.SourceModeXtream, nowUnix)
	case config.SourceModeM3UXMLTV:
		playlistData, err := s.fetchURL(ctx, settings.M3UURL)
		if err != nil {
			s.store.RecordFailure(nowUnix, err.Error())
			return err
		}
		xmltvData, err := s.fetchURL(ctx, settings.EPGXMLURL)
		if err != nil {
			s.store.RecordFailure(nowUnix, err.Error())
			return err
		}
		entries, err := m3u.Parse(playlistData)
		if err != nil {
			s.store.RecordFailure(nowUnix, err.Error())
			return err
		}
		doc, err := xmltv.Parse(xmltvData)
		if err != nil {
			s.store.RecordFailure(nowUnix, err.Error())
			return err
		}
		channels := make([]model.Channel, 0, len(entries))
		programs := make([]model.Program, 0)
		for _, entry := range entries {
			channel := mapping.MapM3UChannel(entry)
			channels = append(channels, channel)
			matchedChannel, ok := matching.Match(entry, doc)
			if !ok {
				continue
			}
			for _, programme := range doc.Programmes {
				if programme.Channel == matchedChannel.ID {
					programs = append(programs, mapping.MapXMLTVProgramme(channel.ID, programme))
				}
			}
		}
		catalog := model.CatalogState{Source: model.LiveTVSource(model.SourceModeM3UXMLTV), Channels: channels, Programs: programs, Health: model.SyncHealth{LastSuccessUnix: nowUnix}}
		state := cache.SnapshotFromCatalog(catalog)
		state.Health.LastSuccessUnix = nowUnix
		s.store.Replace(state)
		return nil
	default:
		return fmt.Errorf("source mode %q not implemented", settings.SourceMode)
	}
}

func (s *Service) syncDispatcharr(ctx context.Context, settings config.Settings, nowUnix int64) error {
	client := s.dispatcharrFactory(settings)

	upstreamChannels, err := client.Channels(ctx)
	if err != nil {
		s.store.RecordFailure(nowUnix, err.Error())
		return err
	}

	groups, err := client.ChannelGroups(ctx)
	if err != nil {
		s.store.RecordFailure(nowUnix, err.Error())
		return err
	}

	content := model.ContentState{LiveCategories: make([]model.Category, 0, len(groups))}
	categoryNames := map[string]string{}
	for _, group := range groups {
		category := mapping.MapDispatcharrCategory(group)
		if category.ID == "" || category.Name == "" {
			continue
		}
		content.LiveCategories = append(content.LiveCategories, category)
		categoryNames[category.ID] = category.Name
	}

	channels := make([]model.Channel, 0, len(upstreamChannels))
	channelByGuideID := map[string]string{}
	for _, upstream := range upstreamChannels {
		if upstream.HiddenFromOutput {
			continue
		}
		channel := mapping.MapDispatcharrChannel(upstream, client.LiveStreamURL(upstream.UUID.String()))
		channel.LogoURL = client.AbsoluteURL(channel.LogoURL)
		channel.CategoryName = categoryNames[channel.CategoryID]
		channels = append(channels, channel)
		if channel.GuideID != "" {
			channelByGuideID[channel.GuideID] = channel.ID
		}
		if upstream.EffectiveEPGDataID.String() != "" {
			channelByGuideID[upstream.EffectiveEPGDataID.String()] = channel.ID
		}
		if upstream.UUID.String() != "" {
			channelByGuideID[upstream.UUID.String()] = channel.ID
		}
	}

	programs := make([]model.Program, 0)
	if upstreamPrograms, err := client.Programs(ctx); err == nil {
		for _, upstream := range upstreamPrograms {
			channelID := channelByGuideID[upstream.TVGID.String()]
			if channelID == "" {
				continue
			}
			programs = append(programs, mapping.MapDispatcharrProgram(channelID, upstream))
		}
	}

	if categories, err := client.VODCategories(ctx); err == nil {
		for _, upstream := range categories {
			category := mapping.MapDispatcharrVODCategory(upstream)
			if category.Kind == "series" {
				content.SeriesCategories = append(content.SeriesCategories, category)
			} else {
				content.VODCategories = append(content.VODCategories, category)
			}
		}
	}
	if movies, err := client.Movies(ctx); err == nil {
		content.VODItems = make([]model.VODItem, 0, len(movies))
		for _, movie := range movies {
			item := mapping.MapDispatcharrMovie(movie, client.MovieStreamURL(movie.UUID.String()))
			item.PosterURL = client.AbsoluteURL(item.PosterURL)
			content.VODItems = append(content.VODItems, item)
		}
	}
	if series, err := client.Series(ctx); err == nil {
		content.SeriesItems = make([]model.SeriesItem, 0, len(series))
		for _, item := range series {
			seriesItem := mapping.MapDispatcharrSeries(item, client.SeriesStreamURL(item.UUID.String()))
			seriesItem.PosterURL = client.AbsoluteURL(seriesItem.PosterURL)
			content.SeriesItems = append(content.SeriesItems, seriesItem)
		}
	}

	catalog := model.CatalogState{
		Source:   model.LiveTVSource(model.SourceModeDirectLogin),
		Channels: channels,
		Programs: programs,
		Health:   model.SyncHealth{LastSuccessUnix: nowUnix},
		Content:  content,
	}
	state := cache.SnapshotFromCatalog(catalog)
	state.Health.LastSuccessUnix = nowUnix
	s.store.Replace(state)
	return nil
}

func (s *Service) syncXtream(ctx context.Context, baseURL, username, password string, sourceMode model.SourceMode, nowUnix int64) error {
	client := s.xtreamFactory(baseURL, username, password)
	streams, err := client.LiveStreams(ctx)
	if err != nil {
		s.store.RecordFailure(nowUnix, err.Error())
		return err
	}

	content := model.ContentState{}
	categoryNames := map[string]string{}
	tightDeadline := hasTightDeadline(ctx)
	if catalogClient, ok := client.(xtreamAppCatalogClient); ok {
		content = loadXtreamAppCatalog(ctx, catalogClient, !tightDeadline)
		for _, category := range content.LiveCategories {
			categoryNames[category.ID] = category.Name
		}
	}

	channels := make([]model.Channel, 0, len(streams))
	programs := make([]model.Program, 0)
	for _, stream := range streams {
		channel := mapping.MapXtreamChannel(stream)
		channel.CategoryName = categoryNames[channel.CategoryID]
		channels = append(channels, channel)

		if tightDeadline {
			continue
		}
		epg, err := client.ShortEPG(ctx, stream.StreamID)
		if err != nil {
			s.store.RecordFailure(nowUnix, err.Error())
			return err
		}
		for _, listing := range epg.EPGListings {
			programs = append(programs, mapping.MapXtreamProgram(channel.ID, listing))
		}
	}

	catalog := model.CatalogState{
		Source:   model.LiveTVSource(sourceMode),
		Channels: channels,
		Programs: programs,
		Health:   model.SyncHealth{LastSuccessUnix: nowUnix},
		Content:  content,
	}
	state := cache.SnapshotFromCatalog(catalog)
	state.Health.LastSuccessUnix = nowUnix
	s.store.Replace(state)
	if tightDeadline {
		settings := config.Settings{SourceMode: config.SourceModeXtream, XtreamBaseURL: baseURL, XtreamUsername: username, XtreamPassword: password}
		if sourceMode == model.SourceModeDirectLogin {
			settings.SourceMode = config.SourceModeDirectLogin
			settings.DispatcharrURL = baseURL
			settings.DispatcharrUser = username
			settings.DispatcharrPass = password
		}
		s.StartAsyncEPGRefresh(settings)
	}
	return nil
}

func loadXtreamAppCatalog(ctx context.Context, client xtreamAppCatalogClient, includeExtended bool) model.ContentState {
	content := model.ContentState{}
	if categories, err := client.LiveCategories(ctx); err == nil {
		content.LiveCategories = make([]model.Category, 0, len(categories))
		for _, category := range categories {
			content.LiveCategories = append(content.LiveCategories, mapping.MapLiveCategory(category))
		}
	}
	if !includeExtended {
		return content
	}
	if categories, err := client.VODCategories(ctx); err == nil {
		content.VODCategories = make([]model.Category, 0, len(categories))
		for _, category := range categories {
			content.VODCategories = append(content.VODCategories, mapping.MapVODCategory(category))
		}
	}
	if streams, err := client.VODStreams(ctx); err == nil {
		content.VODItems = make([]model.VODItem, 0, len(streams))
		for _, stream := range streams {
			content.VODItems = append(content.VODItems, mapping.MapVODItem(stream))
		}
	}
	if categories, err := client.SeriesCategories(ctx); err == nil {
		content.SeriesCategories = make([]model.Category, 0, len(categories))
		for _, category := range categories {
			content.SeriesCategories = append(content.SeriesCategories, mapping.MapSeriesCategory(category))
		}
	}
	if series, err := client.Series(ctx); err == nil {
		content.SeriesItems = make([]model.SeriesItem, 0, len(series))
		for _, item := range series {
			content.SeriesItems = append(content.SeriesItems, mapping.MapSeriesItem(item))
		}
	}
	return content
}

func hasTightDeadline(ctx context.Context) bool {
	deadline, ok := ctx.Deadline()
	return ok && time.Until(deadline) < 45*time.Second
}

func (s *Service) StartAsyncEPGRefresh(settings config.Settings) {
	baseURL, username, password := epgConnectionSettings(settings)
	if baseURL == "" || username == "" || password == "" {
		return
	}

	s.epgMu.Lock()
	if s.epgRunning {
		s.epgMu.Unlock()
		return
	}
	s.epgRunning = true
	s.epgMu.Unlock()

	s.store.MarkEPGLoading()
	go func() {
		defer func() {
			s.epgMu.Lock()
			s.epgRunning = false
			s.epgMu.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := s.refreshEPG(ctx, baseURL, username, password, time.Now().Unix()); err != nil {
			s.store.RecordEPGFailure(time.Now().Unix(), err.Error())
		}
	}()
}

func epgConnectionSettings(settings config.Settings) (string, string, string) {
	if settings.SourceMode == config.SourceModeDirectLogin {
		return settings.DispatcharrURL, settings.DispatcharrUser, settings.DispatcharrPass
	}
	if settings.SourceMode == config.SourceModeXtream {
		return settings.XtreamBaseURL, settings.XtreamUsername, settings.XtreamPassword
	}
	return "", "", ""
}

func (s *Service) refreshEPG(ctx context.Context, baseURL, username, password string, nowUnix int64) error {
	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("parse epg base url: %w", err)
	}
	endpoint.Path = "/xmltv.php"
	query := endpoint.Query()
	query.Set("username", username)
	query.Set("password", password)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("build epg request: %w", err)
	}
	response, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		return fmt.Errorf("fetch epg xmltv: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("fetch epg xmltv status %d", response.StatusCode)
	}
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read epg xmltv: %w", err)
	}
	doc, err := xmltv.Parse(data)
	if err != nil {
		return fmt.Errorf("parse epg xmltv: %w", err)
	}

	snapshot := s.store.Current()
	channelByGuideID := map[string]string{}
	for _, channel := range snapshot.Catalog.Channels {
		if channel.GuideID != "" {
			channelByGuideID[channel.GuideID] = channel.ID
		}
	}
	programs := make([]model.Program, 0, len(doc.Programmes))
	for _, programme := range doc.Programmes {
		channelID := channelByGuideID[programme.Channel]
		if channelID == "" {
			continue
		}
		programs = append(programs, mapping.MapXMLTVProgramme(channelID, programme))
	}
	s.store.ReplacePrograms(programs, nowUnix)
	return nil
}
