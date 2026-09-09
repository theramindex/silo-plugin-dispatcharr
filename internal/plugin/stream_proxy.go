package plugin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	sharedhttp "github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/httpclient"
)

const (
	streamTicketTTL       = 2 * time.Minute
	streamTicketLimit     = 512
	streamProxyMediaBytes = 32 << 20
)

var xtreamCredentialPath = regexp.MustCompile(`(?i)^/(live|movie|series)/[^/]+/[^/]+/`)

type streamTicket struct {
	URL       string
	ExpiresAt time.Time
}

func (s *HTTPRoutesServer) serveProviderStream(ctx context.Context, streamURL string, request *pluginv1.HandleHTTPRequest) *pluginv1.HandleHTTPResponse {
	streamURL = appendPlaybackQuery(streamURL, request)
	settings := config.Settings{}
	if s.settingsProvider != nil {
		settings = s.settingsProvider()
	}
	if streamURLExposesSecrets(streamURL, settings) {
		return s.proxyProviderMedia(ctx, streamURL)
	}
	return redirectResponse(streamURL)
}

func streamURLExposesSecrets(streamURL string, settings config.Settings) bool {
	parsed, err := url.Parse(streamURL)
	if err != nil {
		return false
	}
	if parsed.User != nil {
		return true
	}
	if xtreamCredentialPath.MatchString(parsed.Path) {
		return true
	}
	query := parsed.Query()
	if strings.TrimSpace(query.Get("username")) != "" && strings.TrimSpace(query.Get("password")) != "" {
		return true
	}
	if settings.SourceMode == config.SourceModeXtream {
		_, username, password := xtreamConnectionSettings(settings)
		if username != "" && password != "" && strings.Contains(streamURL, "/"+username+"/"+password+"/") {
			return true
		}
	}
	return false
}

func (s *HTTPRoutesServer) proxyProviderMedia(ctx context.Context, streamURL string) *pluginv1.HandleHTTPResponse {
	body, contentType, finalURL, err := fetchProviderMedia(ctx, streamURL)
	if err != nil {
		return textResponse(http.StatusBadGateway, "stream proxy unavailable")
	}
	if looksLikeHLS(body, contentType) {
		rewritten, err := s.rewriteHLSPlaylist(finalURL, body)
		if err != nil {
			return textResponse(http.StatusBadGateway, "stream proxy unavailable")
		}
		return mediaHTTPResponse(http.StatusOK, "application/vnd.apple.mpegurl", rewritten, "no-store")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return mediaHTTPResponse(http.StatusOK, contentType, body, "private, no-store")
}

func (s *HTTPRoutesServer) handleStreamAsset(ctx context.Context, request *pluginv1.HandleHTTPRequest) *pluginv1.HandleHTTPResponse {
	if request.GetMethod() != "" && request.GetMethod() != http.MethodGet {
		return textResponse(http.StatusMethodNotAllowed, "method not allowed")
	}
	streamURL := s.streamTicketURL(queryValue(request, "ticket"))
	if streamURL == "" {
		return textResponse(http.StatusNotFound, "stream ticket not found")
	}
	return s.proxyProviderMedia(ctx, streamURL)
}

func (s *HTTPRoutesServer) rewriteHLSPlaylist(playlistURL string, body []byte) ([]byte, error) {
	base, err := url.Parse(playlistURL)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(body), "\n")
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		ref, err := url.Parse(trimmed)
		if err != nil {
			continue
		}
		ticket := s.issueStreamTicket(base.ResolveReference(ref).String())
		lines[index] = "/dispatcharr/stream/asset?ticket=" + ticket
	}
	return []byte(strings.Join(lines, "\n")), nil
}

func (s *HTTPRoutesServer) issueStreamTicket(streamURL string) string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		sum := time.Now().UnixNano()
		buf = []byte{byte(sum), byte(sum >> 8), byte(sum >> 16), byte(sum >> 24)}
		buf = append(buf, make([]byte, 12)...)
	}
	ticket := hex.EncodeToString(buf)
	now := time.Now()
	s.streamTicketsMu.Lock()
	defer s.streamTicketsMu.Unlock()
	if s.streamTickets == nil {
		s.streamTickets = map[string]streamTicket{}
	}
	for key, item := range s.streamTickets {
		if now.After(item.ExpiresAt) {
			delete(s.streamTickets, key)
		}
	}
	for len(s.streamTickets) >= streamTicketLimit {
		var oldestKey string
		var oldest time.Time
		for key, item := range s.streamTickets {
			if oldestKey == "" || item.ExpiresAt.Before(oldest) {
				oldestKey, oldest = key, item.ExpiresAt
			}
		}
		delete(s.streamTickets, oldestKey)
	}
	s.streamTickets[ticket] = streamTicket{URL: streamURL, ExpiresAt: now.Add(streamTicketTTL)}
	return ticket
}

func (s *HTTPRoutesServer) streamTicketURL(ticket string) string {
	ticket = strings.TrimSpace(ticket)
	if ticket == "" {
		return ""
	}
	now := time.Now()
	s.streamTicketsMu.Lock()
	defer s.streamTicketsMu.Unlock()
	item, ok := s.streamTickets[ticket]
	if !ok || now.After(item.ExpiresAt) {
		delete(s.streamTickets, ticket)
		return ""
	}
	return item.URL
}

func fetchProviderMedia(ctx context.Context, streamURL string) ([]byte, string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return nil, "", "", err
	}
	response, err := sharedhttp.New().Do(req)
	if err != nil {
		return nil, "", "", sharedhttp.RedactErrorURL(err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, "", "", errStatus(response.StatusCode)
	}
	body, err := sharedhttp.ReadAllLimit(response.Body, streamProxyMediaBytes)
	if err != nil {
		return nil, "", "", err
	}
	contentType := response.Header.Get("Content-Type")
	finalURL := streamURL
	if response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL.String()
	}
	return body, contentType, finalURL, nil
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
	trimmed := strings.TrimSpace(string(body))
	return strings.HasPrefix(trimmed, "#EXTM3U")
}
