package plugin

import (
	"encoding/json"
	"testing"
)

func TestESPNSummaryProbablesAcceptObjectStatistics(t *testing.T) {
	t.Parallel()
	var summary espnStatsSummary
	raw := `{"header":{"id":"1","competitions":[{"status":{"type":{"state":"pre"}},"competitors":[{"homeAway":"home","team":{"id":"10"},"probables":[{"athlete":{"displayName":"Gerrit Cole"},"record":"14-6","statistics":{"splits":{"categories":[]}}}]}]}]}}`
	if err := json.Unmarshal([]byte(raw), &summary); err != nil {
		t.Fatalf("summary probables with object statistics must decode: %v", err)
	}
	result := SportsGameStats{}
	applyESPNCompetitionDetail(&result, summary.Header.Competitions[0], "baseball/mlb")
	if len(result.Probables) != 1 || result.Probables[0].Summary != "14-6" {
		t.Fatalf("unexpected probables %+v", result.Probables)
	}
}
