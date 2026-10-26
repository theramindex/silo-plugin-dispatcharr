package plugin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/transcode"
	sharedhttp "github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/httpclient"
)

// Identity comes from Silo's authenticated proxy, which strips caller-supplied
// X-Silo-* headers before injecting the active account and validated profile.
func transcodeOwner(request *pluginv1.HandleHTTPRequest) string {
	user, profile := "", ""
	for key, value := range request.GetHeaders() {
		if strings.EqualFold(key, "X-Silo-User-Id") {
			user = value
		}
		if strings.EqualFold(key, "X-Silo-Profile-Id") {
			profile = value
		}
	}
	id, err := strconv.Atoi(user)
	if err != nil || id <= 0 {
		return ""
	}
	return strconv.Itoa(id) + ":" + profile
}

func (s *HTTPRoutesServer) transcodeManager() *transcode.Manager {
	s.transcodeMu.Lock()
	defer s.transcodeMu.Unlock()
	if s.transcoder == nil {
		s.transcoder = transcode.New(transcode.Config{})
	}
	return s.transcoder
}

func (s *HTTPRoutesServer) handleTranscode(request *pluginv1.HandleHTTPRequest) *pluginv1.HandleHTTPResponse {
	owner := transcodeOwner(request)
	if owner == "" {
		return textResponse(http.StatusUnauthorized, "Silo sign-in is required for playback conversion")
	}
	manager := s.transcodeManager()
	switch request.GetPath() {
	case "/dispatcharr/api/transcode/start":
		if request.GetMethod() != http.MethodPost {
			return textResponse(405, "method not allowed")
		}
		var payload struct {
			ChannelID string `json:"channelId"`
		}
		if json.Unmarshal(request.GetBody(), &payload) != nil || payload.ChannelID == "" {
			return textResponse(400, "missing channelId")
		}
		source, err := s.resolveStreamURL(payload.ChannelID)
		if err != nil {
			return textResponse(404, "channel not found")
		}
		// Only administrator-configured source hosts may resolve to private IPs.
		// Playlist entries and every redirect remain subject to the dial-time guard.
		client := sharedhttp.NewRestricted(configuredStreamHosts(s.streamProxySettings())...)
		client.Timeout = 0
		if transport, ok := client.Transport.(*http.Transport); ok {
			transport.ResponseHeaderTimeout = 10 * time.Second
		}
		status, err := manager.Start(owner, payload.ChannelID, source, client)
		if err != nil {
			code := http.StatusServiceUnavailable
			if errors.Is(err, transcode.ErrCapacity) {
				code = http.StatusTooManyRequests
			}
			return jsonHTTPResponse(code, map[string]string{"error": err.Error()})
		}
		return jsonHTTPResponse(http.StatusAccepted, status)
	case "/dispatcharr/api/transcode/status":
		if request.GetMethod() != http.MethodGet {
			return textResponse(405, "method not allowed")
		}
		status, err := manager.Status(queryValue(request, "session_id"), owner)
		if err != nil {
			return textResponse(404, "playback conversion session not found")
		}
		return jsonHTTPResponse(200, status)
	case "/dispatcharr/api/transcode/stop":
		if request.GetMethod() != http.MethodPost {
			return textResponse(405, "method not allowed")
		}
		var payload struct {
			ID string `json:"sessionId"`
		}
		if json.Unmarshal(request.GetBody(), &payload) != nil || payload.ID == "" {
			return textResponse(400, "missing sessionId")
		}
		if err := manager.Stop(payload.ID, owner); err != nil {
			return textResponse(404, "playback conversion session not found")
		}
		return jsonHTTPResponse(200, map[string]bool{"ok": true})
	case "/dispatcharr/transcode/media":
		if request.GetMethod() != http.MethodGet {
			return textResponse(405, "method not allowed")
		}
		id, name := queryValue(request, "session_id"), queryValue(request, "name")
		body, err := manager.Media(id, owner, name)
		if err != nil {
			return textResponse(404, "playback conversion media not found")
		}
		if name == "index.m3u8" {
			lines := strings.Split(string(body), "\n")
			for i, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "#") {
					lines[i] = "media?session_id=" + id + "&name=" + line
				}
			}
			return mediaHTTPResponse(200, "application/vnd.apple.mpegurl", []byte(strings.Join(lines, "\n")), "no-store")
		}
		return mediaHTTPResponse(200, "video/mp2t", body, "no-store")
	}
	return textResponse(404, "route not found")
}
