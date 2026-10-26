package plugin

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	sharedhttp "github.com/theramindex/silo-plugins/pkg/livemedia/httpclient"
)

const (
	// Tickets live as long as their playlist keeps being used, so a long VOD
	// can be paused and resumed; idle playlists expire after this window.
	streamTicketIdleTTL = 4 * time.Hour
	// A live playlist slides forward; segments it stopped listing this long
	// ago are dropped on the next rewrite of the same playlist.
	streamTicketStaleAfter = 10 * time.Minute
	streamTicketLimit      = 50000
	// The route SDK buffers whole responses, so proxied media must be bounded.
	streamProxyMediaBytes    = 32 << 20
	streamProxyPlaylistBytes = 8 << 20
	streamRequiresHLSCode    = "stream_requires_hls"
	streamRequiresHLSMessage = "This stream hides provider credentials, so Silo has to relay it, but it is a continuous MPEG-TS or large file " +
		"that cannot be relayed. Configure HLS output for this source (for example an .m3u8 output profile) or switch to Dispatcharr Direct Connect."
)

var (
	xtreamCredentialPath = regexp.MustCompile(`(?i)^/(live|movie|series|timeshift)/[^/]+/[^/]+/`)
	// Xtream-style /USER/PASS/12345[.ts] paths, served by many M3U providers.
	xtreamStylePath = regexp.MustCompile(`^/[^/]+/[^/]+/[0-9]+(\.[A-Za-z0-9]+)?$`)
	hlsURIAttribute = regexp.MustCompile(`URI="([^"]*)"`)
	streamTicketKey = func() []byte {
		key := make([]byte, 32)
		_, _ = rand.Read(key)
		return key
	}()
	errStreamRequiresHLS = errors.New(streamRequiresHLSCode)
)

// sensitiveQueryKeys are compared case-insensitively with "-" folded to "_".
var sensitiveQueryKeys = map[string]bool{
	"password": true, "pass": true, "passwd": true, "pwd": true, "secret": true,
	"token": true, "access_token": true, "auth": true, "auth_token": true, "authorization": true,
	"api_key": true, "apikey": true, "key": true, "signature": true, "sig": true,
}

type streamTicket struct {
	URL      string
	LastSeen time.Time
	group    *streamTicketGroup
}

// streamTicketGroup holds every ticket minted from one upstream playlist.
// Access to any ticket refreshes the whole group, and eviction removes whole
// groups, least recently used first, never the group being rewritten.
type streamTicketGroup struct {
	trustedHost string
	lastAccess  time.Time
	tickets     map[string]struct{}
}

// Browsers must load public provider URLs without Silo credentials: a
// credentialed request that follows a redirect to a provider answering
// Access-Control-Allow-Origin "*" is rejected by CORS.
type resolvedStream struct {
	URL string `json:"url"`
}

type streamProxyError struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

func (s *HTTPRoutesServer) serveProviderStream(ctx context.Context, streamURL string, request *pluginv1.HandleHTTPRequest) *pluginv1.HandleHTTPResponse {
	streamURL = appendPlaybackQuery(streamURL, request)
	settings := s.streamProxySettings()
	exposesSecrets := streamURLExposesSecrets(streamURL, settings)
	if strings.EqualFold(strings.TrimSpace(queryValue(request, "resolve")), "json") {
		if exposesSecrets {
			return jsonHTTPResponse(http.StatusOK, resolvedStream{})
		}
		return jsonHTTPResponse(http.StatusOK, resolvedStream{URL: streamURL})
	}
	if exposesSecrets {
		// The catalog stream URL itself is trusted; URLs found inside the
		// playlists it returns are only trusted on the same or configured hosts.
		return s.proxyProviderMedia(ctx, streamURL, urlHost(streamURL), false)
	}
	return redirectResponse(streamURL)
}

func (s *HTTPRoutesServer) streamProxySettings() config.Settings {
	if s.settingsProvider == nil {
		return config.Settings{}
	}
	return s.settingsProvider()
}

// streamURLExposesSecrets errs on the side of proxying: a false positive costs
// a relay, a false negative hands provider credentials to the browser.
func streamURLExposesSecrets(streamURL string, settings config.Settings) bool {
	parsed, err := url.Parse(streamURL)
	if err != nil {
		return true
	}
	if parsed.User != nil {
		return true
	}
	if xtreamCredentialPath.MatchString(parsed.Path) || xtreamStylePath.MatchString(parsed.Path) {
		return true
	}
	query := parsed.Query()
	for key, values := range query {
		normalized := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(key)), "-", "_")
		if sensitiveQueryKeys[normalized] && strings.TrimSpace(strings.Join(values, "")) != "" {
			return true
		}
	}
	if strings.TrimSpace(query.Get("username")) != "" && strings.TrimSpace(query.Get("password")) != "" {
		return true
	}
	segments := strings.Split(parsed.Path, "/")
	for _, secret := range configuredStreamSecrets(settings) {
		for _, segment := range segments {
			if segment == secret {
				return true
			}
		}
		for _, values := range query {
			for _, value := range values {
				if value == secret {
					return true
				}
			}
		}
		// Substring matches on very short secrets would flag unrelated URLs.
		if len(secret) < 6 {
			continue
		}
		for _, form := range []string{secret, url.PathEscape(secret), url.QueryEscape(secret)} {
			if strings.Contains(streamURL, form) {
				return true
			}
		}
	}
	return false
}

func configuredStreamSecrets(settings config.Settings) []string {
	var secrets []string
	for _, value := range []string{settings.XtreamPassword, settings.DispatcharrPass, settings.DispatcharrAPIKey} {
		if value = strings.TrimSpace(value); value != "" {
			secrets = append(secrets, value)
		}
	}
	return secrets
}

// configuredStreamHosts are the admin-configured source servers; they may be
// private LAN addresses, so the proxy's SSRF guard lets them through.
func configuredStreamHosts(settings config.Settings) []string {
	var hosts []string
	for _, raw := range []string{settings.DispatcharrURL, settings.XtreamBaseURL, settings.M3UURL} {
		if host := urlHost(raw); host != "" {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

func urlHost(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

func (s *HTTPRoutesServer) proxyProviderMedia(ctx context.Context, streamURL, trustedHost string, allowUnsized bool) *pluginv1.HandleHTTPResponse {
	hosts := append(configuredStreamHosts(s.streamProxySettings()), trustedHost)
	media, err := fetchProviderMedia(ctx, s.streamClients.For(hosts...), streamURL, allowUnsized)
	if errors.Is(err, errStreamRequiresHLS) {
		return jsonHTTPResponse(http.StatusUnprocessableEntity, streamProxyError{Code: streamRequiresHLSCode, Error: streamRequiresHLSMessage})
	}
	if err != nil {
		return textResponse(http.StatusBadGateway, "stream proxy unavailable")
	}
	if media.hls {
		rewritten, err := s.rewriteHLSPlaylist(media.finalURL, trustedHost, media.body)
		if err != nil {
			return textResponse(http.StatusBadGateway, "stream proxy unavailable")
		}
		return mediaHTTPResponse(http.StatusOK, "application/vnd.apple.mpegurl", rewritten, "no-store")
	}
	contentType := media.contentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return mediaHTTPResponse(http.StatusOK, contentType, media.body, "private, no-store")
}

func (s *HTTPRoutesServer) handleStreamAsset(ctx context.Context, request *pluginv1.HandleHTTPRequest) *pluginv1.HandleHTTPResponse {
	if request.GetMethod() != "" && request.GetMethod() != http.MethodGet {
		return textResponse(http.StatusMethodNotAllowed, "method not allowed")
	}
	streamURL, trustedHost := s.streamTicketURL(queryValue(request, "ticket"))
	if streamURL == "" {
		return textResponse(http.StatusNotFound, "stream ticket not found")
	}
	// HLS segments are short files, but some servers send them chunked.
	return s.proxyProviderMedia(ctx, streamURL, trustedHost, true)
}

func (s *HTTPRoutesServer) rewriteHLSPlaylist(playlistURL, trustedHost string, body []byte) ([]byte, error) {
	base, err := url.Parse(playlistURL)
	if err != nil {
		return nil, err
	}
	resolve := func(raw string) (string, bool) {
		ref, err := url.Parse(strings.TrimSpace(raw))
		if err != nil {
			return "", false
		}
		target := base.ResolveReference(ref)
		if sharedhttp.CheckScheme(target.Scheme) != nil {
			return "", false
		}
		return target.String(), true
	}
	lines := strings.Split(string(body), "\n")
	// walk calls replace for every fetchable URI in playlist order. URI lines
	// that are not http(s) are dropped rather than passed to the browser.
	walk := func(replace func(target string) string) []string {
		out := make([]string, len(lines))
		for index, line := range lines {
			trimmed := strings.TrimSpace(line)
			switch {
			case trimmed == "":
				out[index] = line
			case strings.HasPrefix(trimmed, "#"):
				// Keys, init maps, and alternate renditions carry URIs in tags.
				out[index] = hlsURIAttribute.ReplaceAllStringFunc(line, func(attribute string) string {
					target, ok := resolve(hlsURIAttribute.FindStringSubmatch(attribute)[1])
					if !ok {
						return attribute
					}
					return `URI="` + replace(target) + `"`
				})
			default:
				if target, ok := resolve(trimmed); ok {
					out[index] = replace(target)
				}
			}
		}
		return out
	}
	var targets []string
	walk(func(target string) string {
		targets = append(targets, target)
		return target
	})
	tickets := s.issueStreamTickets(playlistURL, trustedHost, targets)
	next := 0
	rewritten := walk(func(string) string {
		ticket := tickets[next]
		next++
		return "/dispatcharr/stream/asset?ticket=" + ticket
	})
	return []byte(strings.Join(rewritten, "\n")), nil
}

// issueStreamTickets mints one ticket per target URL, grouped by playlist.
// Ticket IDs are keyed by playlist and target, so re-fetching a live playlist
// reuses the tickets it already handed out instead of growing the store.
func (s *HTTPRoutesServer) issueStreamTickets(playlistURL, trustedHost string, targets []string) []string {
	now := time.Now()
	s.streamTicketsMu.Lock()
	defer s.streamTicketsMu.Unlock()
	if s.streamTickets == nil {
		s.streamTickets = map[string]streamTicket{}
	}
	s.expireStreamTicketsLocked(now)

	ids := make([]string, len(targets))
	var group *streamTicketGroup
	for index, target := range targets {
		ids[index] = streamTicketID(playlistURL, target)
		if existing, ok := s.streamTickets[ids[index]]; ok && group == nil {
			group = existing.group
		}
	}
	if group == nil {
		group = &streamTicketGroup{trustedHost: trustedHost, tickets: map[string]struct{}{}}
	}
	group.lastAccess = now
	for index, target := range targets {
		s.streamTickets[ids[index]] = streamTicket{URL: target, LastSeen: now, group: group}
		group.tickets[ids[index]] = struct{}{}
	}
	for id := range group.tickets {
		item, ok := s.streamTickets[id]
		if !ok || item.group != group {
			delete(group.tickets, id)
			continue
		}
		if now.Sub(item.LastSeen) > streamTicketStaleAfter {
			delete(s.streamTickets, id)
			delete(group.tickets, id)
		}
	}
	s.evictStreamTicketsLocked(group)
	return ids
}

func (s *HTTPRoutesServer) streamTicketURL(ticket string) (string, string) {
	ticket = strings.TrimSpace(ticket)
	if ticket == "" {
		return "", ""
	}
	now := time.Now()
	s.streamTicketsMu.Lock()
	defer s.streamTicketsMu.Unlock()
	item, ok := s.streamTickets[ticket]
	if !ok {
		return "", ""
	}
	if now.Sub(item.group.lastAccess) > streamTicketIdleTTL {
		s.dropStreamTicketGroupLocked(item.group)
		return "", ""
	}
	item.LastSeen = now
	item.group.lastAccess = now
	s.streamTickets[ticket] = item
	return item.URL, item.group.trustedHost
}

func (s *HTTPRoutesServer) expireStreamTicketsLocked(now time.Time) {
	for _, group := range s.streamTicketGroupsLocked(nil) {
		if now.Sub(group.lastAccess) > streamTicketIdleTTL {
			s.dropStreamTicketGroupLocked(group)
		}
	}
}

// evictStreamTicketsLocked enforces the global cap by dropping whole
// playlists, least recently used first. keep is never evicted, even if it
// alone exceeds the cap, so a playlist cannot lose tickets mid-rewrite.
func (s *HTTPRoutesServer) evictStreamTicketsLocked(keep *streamTicketGroup) {
	if len(s.streamTickets) <= streamTicketLimit {
		return
	}
	groups := s.streamTicketGroupsLocked(keep)
	sort.Slice(groups, func(i, j int) bool { return groups[i].lastAccess.Before(groups[j].lastAccess) })
	for _, group := range groups {
		if len(s.streamTickets) <= streamTicketLimit {
			return
		}
		s.dropStreamTicketGroupLocked(group)
	}
}

func (s *HTTPRoutesServer) streamTicketGroupsLocked(skip *streamTicketGroup) []*streamTicketGroup {
	seen := map[*streamTicketGroup]bool{}
	var groups []*streamTicketGroup
	for _, item := range s.streamTickets {
		if item.group == skip || seen[item.group] {
			continue
		}
		seen[item.group] = true
		groups = append(groups, item.group)
	}
	return groups
}

func (s *HTTPRoutesServer) dropStreamTicketGroupLocked(group *streamTicketGroup) {
	for id := range group.tickets {
		if item, ok := s.streamTickets[id]; ok && item.group == group {
			delete(s.streamTickets, id)
		}
	}
	group.tickets = map[string]struct{}{}
}

func streamTicketID(playlistURL, target string) string {
	mac := hmac.New(sha256.New, streamTicketKey)
	mac.Write([]byte(playlistURL))
	mac.Write([]byte{0})
	mac.Write([]byte(target))
	return hex.EncodeToString(mac.Sum(nil)[:16])
}

type providerMedia struct {
	body        []byte
	contentType string
	finalURL    string
	hls         bool
}

// fetchProviderMedia relays HLS playlists and bounded media. Endless live
// MPEG-TS and large files cannot pass through the buffered route SDK, so they
// fail fast with errStreamRequiresHLS instead of hanging until a timeout.
func fetchProviderMedia(ctx context.Context, client *http.Client, streamURL string, allowUnsized bool) (providerMedia, error) {
	parsed, err := url.Parse(streamURL)
	if err != nil {
		return providerMedia{}, err
	}
	if err := sharedhttp.CheckScheme(parsed.Scheme); err != nil {
		return providerMedia{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return providerMedia{}, err
	}
	response, err := client.Do(req)
	if err != nil {
		return providerMedia{}, sharedhttp.RedactErrorURL(err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return providerMedia{}, errStatus(response.StatusCode)
	}
	media := providerMedia{contentType: response.Header.Get("Content-Type"), finalURL: streamURL}
	if response.Request != nil && response.Request.URL != nil {
		media.finalURL = response.Request.URL.String()
	}
	reader := bufio.NewReaderSize(response.Body, 4096)
	media.hls = looksLikeHLS(peekPrefix(reader), media.contentType)
	limit := int64(streamProxyMediaBytes)
	if media.hls {
		limit = streamProxyPlaylistBytes
	} else if response.ContentLength > limit || (response.ContentLength < 0 && !allowUnsized) {
		return providerMedia{}, errStreamRequiresHLS
	}
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return providerMedia{}, err
	}
	if int64(len(body)) > limit {
		if media.hls {
			return providerMedia{}, errors.New("playlist too large")
		}
		return providerMedia{}, errStreamRequiresHLS
	}
	media.body = body
	return media, nil
}

// peekPrefix returns the first bytes of the body without consuming them; a
// live stream yields its first packets immediately, so this does not block.
func peekPrefix(reader *bufio.Reader) []byte {
	prefix, _ := reader.Peek(64)
	return prefix
}

type statusError int

func errStatus(code int) error {
	return statusError(code)
}

func (s statusError) Error() string {
	return http.StatusText(int(s))
}

func looksLikeHLS(body []byte, contentType string) bool {
	if strings.Contains(strings.ToLower(contentType), "mpegurl") {
		return true
	}
	trimmed := bytes.TrimSpace(bytes.TrimPrefix(body, []byte("\xef\xbb\xbf")))
	return bytes.HasPrefix(trimmed, []byte("#EXTM3U"))
}
