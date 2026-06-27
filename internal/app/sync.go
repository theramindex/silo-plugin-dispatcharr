package app

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
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
		return s.syncDispatcharr(ctx, settings, nowUnix)
	case config.SourceModeXtream:
		return s.syncXtream(ctx, settings, model.SourceModeXtream, nowUnix)
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
		catalog := model.CatalogState{Source: model.LiveTVSource(model.SourceModeM3UXMLTV), Channels: channels, Programs: programs, Health: syncHealth(nowUnix, len(programs))}
		state := cache.SnapshotFromCatalog(catalog)
		state.Health.LastSuccessUnix = nowUnix
		state.ConfigKey = config.CatalogCacheKey(settings)
		s.replaceSnapshot(state)
		return nil
	default:
		return fmt.Errorf("source mode %q not implemented", settings.SourceMode)
	}
}

func (s *Service) syncDispatcharr(ctx context.Context, settings config.Settings, nowUnix int64) error {
	client := s.dispatcharrFactory(settings)
	tightDeadline := hasTightDeadline(ctx)

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
	channelByUpstreamID := map[string]string{}
	for _, upstream := range upstreamChannels {
		if upstream.HiddenFromOutput {
			continue
		}
		channel := mapping.MapDispatcharrChannel(upstream, client.LiveStreamURL(upstream.UUID.String()))
		channel.LogoURL = client.AbsoluteURL(channel.LogoURL)
		if channel.LogoURL == "" {
			channel.LogoURL = client.LogoCacheURL(firstPresent(upstream.EffectiveLogoID.String(), upstream.LogoID.String()))
		}
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
		if upstream.ID.String() != "" {
			channelByUpstreamID[upstream.ID.String()] = channel.ID
		}
	}
	sortChannelsByLineupNumber(channels)

	programs := make([]model.Program, 0)
	programIDs := map[string]struct{}{}
	if upstreamPrograms, err := client.Programs(ctx); err == nil {
		for _, upstream := range upstreamPrograms {
			channelID := channelByGuideID[upstream.TVGID.String()]
			if channelID == "" {
				continue
			}
			program := mapping.MapDispatcharrProgram(channelID, upstream)
			programs = append(programs, program)
			programIDs[program.ID] = struct{}{}
		}
	}
	if !tightDeadline {
		start := time.Unix(nowUnix, 0).Add(-1 * time.Hour)
		end := time.Unix(nowUnix, 0).Add(24 * time.Hour)
		if upstreamPrograms, err := client.SearchPrograms(ctx, start, end); err == nil {
			for _, upstream := range upstreamPrograms {
				channelID := ""
				for _, channel := range upstream.Channels {
					if mapped := channelByUpstreamID[channel.ID.String()]; mapped != "" {
						channelID = mapped
						break
					}
				}
				if channelID == "" {
					continue
				}
				program := mapping.MapDispatcharrProgram(channelID, upstream.Program)
				if _, ok := programIDs[program.ID]; ok {
					continue
				}
				programs = append(programs, program)
				programIDs[program.ID] = struct{}{}
			}
		}
	}

	if !tightDeadline {
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
	}

	catalog := model.CatalogState{
		Source:   model.LiveTVSource(model.SourceModeDirectLogin),
		Channels: channels,
		Programs: programs,
		Health:   syncHealth(nowUnix, len(programs)),
		Content:  content,
	}
	state := cache.SnapshotFromCatalog(catalog)
	state.Health.LastSuccessUnix = nowUnix
	state.ConfigKey = config.CatalogCacheKey(settings)
	s.replaceSnapshot(state)
	if tightDeadline || len(programs) == 0 {
		s.StartAsyncEPGRefresh(settings)
	}
	return nil
}

func (s *Service) syncXtream(ctx context.Context, settings config.Settings, sourceMode model.SourceMode, nowUnix int64) error {
	baseURL, username, password := xtreamConnectionSettings(settings)
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
	}

	if !tightDeadline && strings.TrimSpace(settings.EPGXMLURL) != "" {
		xmltvPrograms, err := s.xmltvProgramsForChannels(ctx, settings.EPGXMLURL, channels)
		if err != nil {
			s.store.RecordFailure(nowUnix, err.Error())
			return err
		}
		programs = append(programs, xmltvPrograms...)
	}

	if len(programs) == 0 {
		for _, stream := range streams {
			if tightDeadline {
				continue
			}
			channel := mapping.MapXtreamChannel(stream)
			epg, err := client.ShortEPG(ctx, stream.StreamID)
			if err != nil {
				s.store.RecordFailure(nowUnix, err.Error())
				return err
			}
			for _, listing := range epg.EPGListings {
				programs = append(programs, mapping.MapXtreamProgram(channel.ID, listing))
			}
		}
	}

	sortChannelsByLineupNumber(channels)

	catalog := model.CatalogState{
		Source:   model.LiveTVSource(sourceMode),
		Channels: channels,
		Programs: programs,
		Health:   syncHealth(nowUnix, len(programs)),
		Content:  content,
	}
	state := cache.SnapshotFromCatalog(catalog)
	state.Health.LastSuccessUnix = nowUnix
	state.ConfigKey = config.CatalogCacheKey(settings)
	s.replaceSnapshot(state)
	if tightDeadline {
		s.StartAsyncEPGRefresh(settings)
	}
	return nil
}

func xtreamConnectionSettings(settings config.Settings) (string, string, string) {
	if settings.SourceMode == config.SourceModeDirectLogin {
		return settings.DispatcharrURL, settings.DispatcharrUser, settings.DispatcharrPass
	}
	return settings.XtreamBaseURL, settings.XtreamUsername, settings.XtreamPassword
}

func (s *Service) xmltvProgramsForChannels(ctx context.Context, rawURL string, channels []model.Channel) ([]model.Program, error) {
	data, err := s.fetchURL(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetch custom xmltv: %w", err)
	}
	doc, err := xmltv.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse custom xmltv: %w", err)
	}
	return programsFromXMLTVDocument(channels, doc), nil
}

func programsFromXMLTVDocument(channels []model.Channel, doc xmltv.Document) []model.Program {
	channelByGuideID := map[string]string{}
	for _, channel := range channels {
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
	return programs
}

func sortChannelsByLineupNumber(channels []model.Channel) {
	sort.SliceStable(channels, func(i, j int) bool {
		leftNumber, leftOK := leadingChannelNumber(channels[i].Number)
		rightNumber, rightOK := leadingChannelNumber(channels[j].Number)
		if leftOK && rightOK && leftNumber != rightNumber {
			return leftNumber < rightNumber
		}
		if leftOK != rightOK {
			return leftOK
		}
		left := strings.TrimSpace(strings.ToLower(channels[i].Number))
		right := strings.TrimSpace(strings.ToLower(channels[j].Number))
		if left != "" && right != "" && left != right {
			return left < right
		}
		return false
	})
}

func leadingChannelNumber(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	end := 0
	dotSeen := false
	for end < len(value) {
		ch := value[end]
		if ch >= '0' && ch <= '9' {
			end++
			continue
		}
		if ch == '.' && !dotSeen {
			dotSeen = true
			end++
			continue
		}
		break
	}
	if end == 0 || value[:end] == "." {
		return 0, false
	}
	number, err := strconv.ParseFloat(value[:end], 64)
	return number, err == nil
}

func firstPresent(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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
	if !usesDispatcharrAPI(settings) {
		if _, err := epgURL(settings); err != nil {
			return
		}
	}

	if usesDispatcharrAPI(settings) {
		s.epgMu.Lock()
		if s.epgRunning {
			s.epgMu.Unlock()
			return
		}
		s.epgRunning = true
		s.epgMu.Unlock()

		s.store.MarkEPGLoading()
		s.persistSnapshot()
		go func() {
			defer func() {
				s.epgMu.Lock()
				s.epgRunning = false
				s.epgMu.Unlock()
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			now := time.Now().Unix()
			if err := s.SyncNow(ctx, settings, now); err != nil {
				s.store.RecordEPGFailure(now, err.Error())
				s.persistSnapshot()
			}
		}()
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
	s.persistSnapshot()
	go func() {
		defer func() {
			s.epgMu.Lock()
			s.epgRunning = false
			s.epgMu.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if err := s.refreshEPG(ctx, settings, time.Now().Unix()); err != nil {
			s.store.RecordEPGFailure(time.Now().Unix(), err.Error())
		}
	}()
}

func (s *Service) RefreshEPGNow(ctx context.Context, settings config.Settings, nowUnix int64) error {
	if err := settings.Validate(); err != nil {
		return err
	}
	s.clearGuidePrograms(nowUnix)
	if usesDispatcharrAPI(settings) {
		if err := s.SyncNow(ctx, settings, nowUnix); err != nil {
			s.store.RecordEPGFailure(nowUnix, err.Error())
			s.persistSnapshot()
			return err
		}
		return nil
	}
	if _, err := epgURL(settings); err != nil {
		return s.SyncNow(ctx, settings, nowUnix)
	}
	if err := s.refreshEPG(ctx, settings, nowUnix); err != nil {
		s.store.RecordEPGFailure(nowUnix, err.Error())
		s.persistSnapshot()
		return err
	}
	return nil
}

func (s *Service) ForceSyncNow(ctx context.Context, settings config.Settings, nowUnix int64) error {
	if err := settings.Validate(); err != nil {
		return err
	}
	s.clearGuidePrograms(nowUnix)
	if err := s.SyncNow(ctx, settings, nowUnix); err != nil {
		s.store.RecordEPGFailure(nowUnix, err.Error())
		s.persistSnapshot()
		return err
	}
	return nil
}

func usesDispatcharrAPI(settings config.Settings) bool {
	return settings.SourceMode == config.SourceModeDirectLogin || settings.SourceMode == config.SourceModeAPIKey
}

func syncHealth(nowUnix int64, programCount int) model.SyncHealth {
	health := model.SyncHealth{LastSuccessUnix: nowUnix}
	if programCount > 0 {
		health.EPGStatus = "ok"
		health.EPGProgramCount = programCount
		health.EPGLastSuccessUnix = nowUnix
	}
	return health
}

func epgURL(settings config.Settings) (string, error) {
	if settings.SourceMode == config.SourceModeM3UXMLTV && strings.TrimSpace(settings.EPGXMLURL) != "" {
		return strings.TrimSpace(settings.EPGXMLURL), nil
	}
	if settings.SourceMode == config.SourceModeXtream && strings.TrimSpace(settings.EPGXMLURL) != "" {
		return strings.TrimSpace(settings.EPGXMLURL), nil
	}
	baseURL, username, password := epgConnectionSettings(settings)
	if baseURL == "" || username == "" || password == "" {
		return "", fmt.Errorf("epg connection settings are required")
	}
	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse epg base url: %w", err)
	}
	endpoint.Path = "/xmltv.php"
	query := endpoint.Query()
	query.Set("username", username)
	query.Set("password", password)
	endpoint.RawQuery = query.Encode()
	return endpoint.String(), nil
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

func (s *Service) refreshEPG(ctx context.Context, settings config.Settings, nowUnix int64) error {
	rawURL, err := epgURL(settings)
	if err != nil {
		return err
	}
	data, err := s.fetchURL(ctx, rawURL)
	if err != nil {
		return fmt.Errorf("fetch epg xmltv: %w", err)
	}
	doc, err := xmltv.Parse(data)
	if err != nil {
		return fmt.Errorf("parse epg xmltv: %w", err)
	}

	snapshot := s.store.Current()
	programs := programsFromXMLTVDocument(snapshot.Catalog.Channels, doc)
	s.replacePrograms(programs, nowUnix)
	return nil
}
