package dispatcharr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	sharedhttp "github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/httpclient"
)

type Client struct {
	baseURL  string
	username string
	password string
	apiKey   string
	http     *http.Client

	mu      sync.Mutex
	access  string
	refresh string
}

func NewLoginClient(baseURL, username, password string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), username: username, password: password, http: sharedhttp.New()}
}

func NewAPIKeyClient(baseURL, apiKey string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, http: sharedhttp.New()}
}

func (c *Client) TestConnection(ctx context.Context) error {
	var target map[string]any
	return c.getJSON(ctx, "/api/accounts/users/me/", &target)
}

func (c *Client) Channels(ctx context.Context) ([]Channel, error) {
	var channels []Channel
	return channels, c.getList(ctx, "/api/channels/channels/", &channels)
}

func (c *Client) ChannelGroups(ctx context.Context) ([]ChannelGroup, error) {
	var groups []ChannelGroup
	return groups, c.getList(ctx, "/api/channels/groups/", &groups)
}

func (c *Client) Programs(ctx context.Context) ([]Program, error) {
	var response struct {
		Data []Program `json:"data"`
	}
	if err := c.getJSON(ctx, "/api/epg/grid/", &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) VODCategories(ctx context.Context) ([]VODCategory, error) {
	var categories []VODCategory
	return categories, c.getList(ctx, "/api/vod/categories/", &categories)
}

func (c *Client) Movies(ctx context.Context) ([]Movie, error) {
	var movies []Movie
	return movies, c.getList(ctx, "/api/vod/movies/", &movies)
}

func (c *Client) Series(ctx context.Context) ([]Series, error) {
	var series []Series
	return series, c.getList(ctx, "/api/vod/series/", &series)
}

func (c *Client) LiveStreamURL(channelUUID string) string {
	return c.absolutePath(path.Join("/proxy/ts/stream", strings.TrimSpace(channelUUID)))
}

func (c *Client) MovieStreamURL(movieUUID string) string {
	return c.absolutePath(path.Join("/proxy/vod/movie", strings.TrimSpace(movieUUID)))
}

func (c *Client) SeriesStreamURL(seriesUUID string) string {
	return c.absolutePath(path.Join("/proxy/vod/series", strings.TrimSpace(seriesUUID)))
}

func (c *Client) AbsoluteURL(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err == nil && parsed.IsAbs() {
		return raw
	}
	return c.absolutePath(raw)
}

func (c *Client) getList(ctx context.Context, endpoint string, target any) error {
	next := endpoint
	for strings.TrimSpace(next) != "" {
		var page struct {
			Next    string          `json:"next"`
			Results json.RawMessage `json:"results"`
		}
		raw, err := c.getRaw(ctx, next)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &page); err == nil && len(page.Results) > 0 {
			if err := appendJSONList(page.Results, target); err != nil {
				return err
			}
			next = page.Next
			continue
		}
		return appendJSONList(raw, target)
	}
	return nil
}

func appendJSONList(raw []byte, target any) error {
	var current []json.RawMessage
	if err := json.Unmarshal(raw, &current); err != nil {
		return fmt.Errorf("decode list: %w", err)
	}

	out, err := json.Marshal(current)
	if err != nil {
		return err
	}

	switch values := target.(type) {
	case *[]Channel:
		var next []Channel
		if err := json.Unmarshal(out, &next); err != nil {
			return err
		}
		*values = append(*values, next...)
	case *[]ChannelGroup:
		var next []ChannelGroup
		if err := json.Unmarshal(out, &next); err != nil {
			return err
		}
		*values = append(*values, next...)
	case *[]VODCategory:
		var next []VODCategory
		if err := json.Unmarshal(out, &next); err != nil {
			return err
		}
		*values = append(*values, next...)
	case *[]Movie:
		var next []Movie
		if err := json.Unmarshal(out, &next); err != nil {
			return err
		}
		*values = append(*values, next...)
	case *[]Series:
		var next []Series
		if err := json.Unmarshal(out, &next); err != nil {
			return err
		}
		*values = append(*values, next...)
	default:
		return fmt.Errorf("unsupported list target %T", target)
	}
	return nil
}

func (c *Client) getJSON(ctx context.Context, endpoint string, target any) error {
	raw, err := c.getRaw(ctx, endpoint)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) getRaw(ctx context.Context, endpoint string) ([]byte, error) {
	return c.getRawWithRetry(ctx, endpoint, true)
}

func (c *Client) getRawWithRetry(ctx context.Context, endpoint string, allowRefresh bool) ([]byte, error) {
	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint(endpoint), nil)
	if err != nil {
		return nil, err
	}
	c.authorize(req)
	response, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer response.Body.Close()
	if allowRefresh && response.StatusCode == http.StatusUnauthorized && c.refresh != "" {
		if err := c.refreshToken(ctx); err == nil {
			return c.getRawWithRetry(ctx, endpoint, false)
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d", response.StatusCode)
	}
	return io.ReadAll(response.Body)
}

func (c *Client) ensureAuth(ctx context.Context) error {
	if strings.TrimSpace(c.apiKey) != "" {
		return nil
	}
	c.mu.Lock()
	hasAccess := c.access != ""
	c.mu.Unlock()
	if hasAccess {
		return nil
	}
	return c.login(ctx)
}

func (c *Client) login(ctx context.Context) error {
	payload, err := json.Marshal(map[string]string{"username": c.username, "password": c.password})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/api/accounts/token/"), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("dispatcharr login status %d", response.StatusCode)
	}
	var token struct {
		Access  string `json:"access"`
		Refresh string `json:"refresh"`
	}
	if err := json.NewDecoder(response.Body).Decode(&token); err != nil {
		return err
	}
	c.mu.Lock()
	c.access = token.Access
	c.refresh = token.Refresh
	c.mu.Unlock()
	return nil
}

func (c *Client) refreshToken(ctx context.Context) error {
	c.mu.Lock()
	refresh := c.refresh
	c.mu.Unlock()
	if refresh == "" {
		return fmt.Errorf("missing refresh token")
	}
	payload, err := json.Marshal(map[string]string{"refresh": refresh})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/api/accounts/token/refresh/"), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("dispatcharr refresh status %d", response.StatusCode)
	}
	var token struct {
		Access string `json:"access"`
	}
	if err := json.NewDecoder(response.Body).Decode(&token); err != nil {
		return err
	}
	c.mu.Lock()
	c.access = token.Access
	c.mu.Unlock()
	return nil
}

func (c *Client) authorize(req *http.Request) {
	if strings.TrimSpace(c.apiKey) != "" {
		req.Header.Set("X-API-Key", c.apiKey)
		req.Header.Set("Authorization", "ApiKey "+c.apiKey)
		return
	}
	c.mu.Lock()
	access := c.access
	c.mu.Unlock()
	if access != "" {
		req.Header.Set("Authorization", "Bearer "+access)
	}
}

func (c *Client) endpoint(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err == nil && parsed.IsAbs() {
		return endpoint
	}
	return c.absolutePath(endpoint)
}

func (c *Client) absolutePath(rawPath string) string {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return ""
	}
	relative, err := url.Parse(rawPath)
	if err == nil && relative.IsAbs() {
		return rawPath
	}
	base.Path = path.Join(base.Path, rawPath)
	return base.String()
}
