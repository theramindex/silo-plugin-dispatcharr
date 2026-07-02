package dispatcharr

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEndpointPreservesTrailingSlash(t *testing.T) {
	t.Parallel()

	client := NewLoginClient("https://dispatcharr.example.com", "demo", "secret")
	if got := client.endpoint("/api/accounts/token/"); got != "https://dispatcharr.example.com/api/accounts/token/" {
		t.Fatalf("unexpected endpoint: %q", got)
	}
}

func TestGetRawRelogsInWhenRefreshTokenExpired(t *testing.T) {
	t.Parallel()

	loginCount := 0
	refreshCount := 0
	resourceCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/accounts/token/":
			loginCount++
			w.Header().Set("Content-Type", "application/json")
			if loginCount == 1 {
				_, _ = w.Write([]byte(`{"access":"expired-access","refresh":"expired-refresh"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access":"fresh-access","refresh":"fresh-refresh"}`))
		case "/api/accounts/token/refresh/":
			refreshCount++
			http.Error(w, "refresh expired", http.StatusUnauthorized)
		case "/api/channels/":
			resourceCount++
			if r.Header.Get("Authorization") != "Bearer fresh-access" {
				http.Error(w, "stale access", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewLoginClient(server.URL, "demo", "secret")
	body, err := client.getRaw(t.Context(), "/api/channels/")
	if err != nil {
		t.Fatalf("get raw: %v", err)
	}
	if string(body) != `[]` {
		t.Fatalf("unexpected body: %s", body)
	}
	if loginCount != 2 {
		t.Fatalf("expected initial login and re-login, got %d", loginCount)
	}
	if refreshCount != 1 {
		t.Fatalf("expected one refresh attempt, got %d", refreshCount)
	}
	if resourceCount != 2 {
		t.Fatalf("expected original request and retry, got %d", resourceCount)
	}
}

func TestRefreshTokenStoresRotatedRefreshToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/accounts/token/":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access":"expired-access","refresh":"old-refresh"}`))
		case "/api/accounts/token/refresh/":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access":"fresh-access","refresh":"new-refresh"}`))
		case "/api/channels/":
			if r.Header.Get("Authorization") != "Bearer fresh-access" {
				http.Error(w, "stale access", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewLoginClient(server.URL, "demo", "secret")
	if _, err := client.getRaw(t.Context(), "/api/channels/"); err != nil {
		t.Fatalf("get raw: %v", err)
	}

	client.mu.Lock()
	refresh := client.refresh
	client.mu.Unlock()
	if refresh != "new-refresh" {
		t.Fatalf("expected rotated refresh token, got %q", refresh)
	}
}
