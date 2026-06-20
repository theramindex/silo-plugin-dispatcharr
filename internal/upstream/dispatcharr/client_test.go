package dispatcharr

import "testing"

func TestEndpointPreservesTrailingSlash(t *testing.T) {
	t.Parallel()

	client := NewLoginClient("https://dispatcharr.example.com", "demo", "secret")
	if got := client.endpoint("/api/accounts/token/"); got != "https://dispatcharr.example.com/api/accounts/token/" {
		t.Fatalf("unexpected endpoint: %q", got)
	}
}
