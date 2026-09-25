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

func TestESPNSummaryHighlightsKeepPlayableStreamAndWebPage(t *testing.T) {
	t.Parallel()
	var summary espnStatsSummary
	raw := `{"header":{"id":"1","competitions":[{"status":{"type":{"state":"post"}}}]},"videos":[
		{"headline":"Game Highlights","thumbnail":"https://a.espncdn.com/thumb.jpg","links":{"source":{"href":"https://media.video-cdn.espn.com/clip.mp4"},"web":{"href":"https://www.espn.com/video/clip?id=1"}}},
		{"headline":"Web only","links":{"web":{"href":"https://www.espn.com/video/clip?id=2"}}},
		{"headline":"Insecure","links":{"source":{"href":"http://media.video-cdn.espn.com/clip.mp4"}}}
	]}`
	if err := json.Unmarshal([]byte(raw), &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	videos := espnGameStats(SportsEvent{}, summary).Videos
	if len(videos) != 2 {
		t.Fatalf("expected two https highlights, got %+v", videos)
	}
	if videos[0].Stream != "https://media.video-cdn.espn.com/clip.mp4" || videos[0].URL != "https://www.espn.com/video/clip?id=1" {
		t.Fatalf("highlight must carry the mp4 stream and the ESPN page, got %+v", videos[0])
	}
	if videos[1].Stream != "" || videos[1].URL != "https://www.espn.com/video/clip?id=2" {
		t.Fatalf("web-only highlight must not claim a stream, got %+v", videos[1])
	}
}
