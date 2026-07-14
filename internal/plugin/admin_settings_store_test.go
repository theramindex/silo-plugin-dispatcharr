package plugin

import (
	"strings"
	"testing"
)

func TestNormalizeAdminSettingsEventKeywordRuleDefaultsAndClamps(t *testing.T) {
	t.Parallel()

	normalized := normalizeAdminSettingsPayload(map[string]any{
		"eventKeywords": []any{
			map[string]any{
				"categoryId":         "  civic ",
				"categoryName":       " Civic ",
				"keywords":           []any{" State of the Union ", ""},
				"excludeKeywords":    []any{"  rehearsal ", ""},
				"eventSeries":        true,
				"groupWindowMinutes": float64(999),
			},
			map[string]any{
				"categoryId":         "parades",
				"keywords":           []any{"Parade"},
				"groupWindowMinutes": float64(1),
			},
		},
	})

	rules := normalized["eventKeywords"].([]map[string]any)
	if len(rules) != 2 {
		t.Fatalf("expected two normalized rules, got %d", len(rules))
	}
	if rules[0]["excludeKeywords"].([]string)[0] != "rehearsal" || rules[0]["eventSeries"] != true || rules[0]["groupWindowMinutes"] != 360 {
		t.Fatalf("expected first rule options to be preserved/clamped, got %+v", rules[0])
	}
	if rules[1]["eventSeries"] != false || rules[1]["groupWindowMinutes"] != 15 {
		t.Fatalf("expected legacy rule defaults and lower clamp, got %+v", rules[1])
	}
}

func TestNormalizeAdminSettingsJSONRoundTripsEventKeywordRuleOptions(t *testing.T) {
	t.Parallel()

	data, err := normalizeAdminSettingsJSON([]byte(`{"eventKeywords":[{"categoryId":"civic","categoryName":"Civic","keywords":["Debate"],"excludeKeywords":["rerun"],"eventSeries":true,"groupWindowMinutes":45}]}`))
	if err != nil {
		t.Fatalf("normalize settings: %v", err)
	}
	if got := string(data); got == "" {
		t.Fatal("expected normalized settings JSON")
	}
	if !containsJSON(data, `"excludeKeywords":["rerun"]`) || !containsJSON(data, `"eventSeries":true`) || !containsJSON(data, `"groupWindowMinutes":45`) {
		t.Fatalf("expected event keyword options to round-trip, got %s", data)
	}
}

func containsJSON(data []byte, fragment string) bool {
	return strings.Contains(string(data), fragment)
}

func TestNormalizeAdminSettingsClampsLiveRewindLimits(t *testing.T) {
	t.Parallel()
	normalized := normalizeAdminSettingsPayload(map[string]any{
		"liveRewindEnabled":       true,
		"liveRewindCacheGB":       float64(900),
		"liveRewindWindowMinutes": float64(45),
		"liveRewindMinFreeGB":     float64(0),
		"liveRewindMaxChannels":   float64(250),
	})
	if normalized["liveRewindEnabled"] != true {
		t.Fatal("expected rewind to remain enabled")
	}
	if normalized["liveRewindCacheGB"] != float64(500) {
		t.Fatalf("expected cache limit clamp, got %v", normalized["liveRewindCacheGB"])
	}
	if normalized["liveRewindWindowMinutes"] != 30 {
		t.Fatalf("expected invalid window to use default, got %v", normalized["liveRewindWindowMinutes"])
	}
	if normalized["liveRewindMinFreeGB"] != float64(1) {
		t.Fatalf("expected free-space clamp, got %v", normalized["liveRewindMinFreeGB"])
	}
	if normalized["liveRewindMaxChannels"] != 100 {
		t.Fatalf("expected channel limit clamp, got %v", normalized["liveRewindMaxChannels"])
	}
}

func TestNormalizeAdminSettingsSportsFirstPlayerEnabledDefaultsFalse(t *testing.T) {
	t.Parallel()

	if normalized := normalizeAdminSettingsPayload(map[string]any{})["sportsFirstPlayerEnabled"]; normalized != false {
		t.Fatalf("expected sports-first player setting to default to false, got %v", normalized)
	}
}

func TestNormalizeAdminSettingsPreservesProfileOnlyHierarchy(t *testing.T) {
	t.Parallel()

	normalized := normalizeAdminSettingsPayload(map[string]any{
		"mode":               "normal",
		"virtualGroupSource": "profile",
	})
	if normalized["virtualGroupSource"] != "profile" {
		t.Fatalf("expected profile-only hierarchy, got %v", normalized["virtualGroupSource"])
	}
	if normalized["mode"] != "delimiter" {
		t.Fatalf("expected profile hierarchy to use delimiter parsing, got %v", normalized["mode"])
	}
}

func TestNormalizeAdminSettingsPreservesSportsFirstPlayerEnabled(t *testing.T) {
	t.Parallel()

	if normalized := normalizeAdminSettingsPayload(map[string]any{"sportsFirstPlayerEnabled": true})["sportsFirstPlayerEnabled"]; normalized != true {
		t.Fatalf("expected sports-first player setting to remain enabled, got %v", normalized)
	}
}

func TestNormalizeAdminSettingsSportsReplayDefaults(t *testing.T) {
	t.Parallel()

	normalized := normalizeAdminSettingsPayload(map[string]any{})
	if normalized["sportsEnabled"] != true {
		t.Fatalf("expected sports replay to default to enabled, got %v", normalized["sportsEnabled"])
	}
	ids, ok := normalized["sportsLibraryIds"].([]int)
	if !ok || len(ids) != 0 {
		t.Fatalf("expected sports library IDs to default to an empty array, got %v", normalized["sportsLibraryIds"])
	}
}

func TestNormalizeAdminSettingsSportsReplayPreservesFalseAndNormalizesLibraryIDs(t *testing.T) {
	t.Parallel()

	normalized := normalizeAdminSettingsPayload(map[string]any{
		"sportsEnabled": false,
		"sportsLibraryIds": []any{
			float64(12), float64(0), float64(-4), float64(12), float64(7.5),
			"19", true, float64(3),
		},
	})
	if normalized["sportsEnabled"] != false {
		t.Fatalf("expected explicit sports disable to be preserved, got %v", normalized["sportsEnabled"])
	}
	ids, ok := normalized["sportsLibraryIds"].([]int)
	if !ok || len(ids) != 2 || ids[0] != 12 || ids[1] != 3 {
		t.Fatalf("expected positive integral library IDs, deduped in order, got %#v", normalized["sportsLibraryIds"])
	}
}

func TestNormalizeAdminSettingsSportsReplayMalformedValuesUseDefaults(t *testing.T) {
	t.Parallel()

	normalized := normalizeAdminSettingsPayload(map[string]any{
		"sportsEnabled":    "false",
		"sportsLibraryIds": "12",
	})
	if normalized["sportsEnabled"] != true {
		t.Fatalf("expected malformed sportsEnabled to use true, got %v", normalized["sportsEnabled"])
	}
	if ids := normalized["sportsLibraryIds"].([]int); len(ids) != 0 {
		t.Fatalf("expected malformed sports library IDs to use an empty array, got %v", ids)
	}
}

func TestNormalizeAdminSettingsAppDisplayName(t *testing.T) {
	t.Parallel()

	longName := strings.Repeat("界", 81)
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{name: "defaults when missing", expected: "Live TV (Dispatcharr)"},
		{name: "defaults when empty", input: "   ", expected: "Live TV (Dispatcharr)"},
		{name: "trims whitespace", input: "  My Live TV  ", expected: "My Live TV"},
		{name: "truncates at 80 characters", input: longName, expected: string([]rune(longName)[:80])},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]any{}
			if tt.input != nil {
				payload["appDisplayName"] = tt.input
			}
			normalized := normalizeAdminSettingsPayload(payload)
			if got := normalized["appDisplayName"]; got != tt.expected {
				t.Fatalf("expected appDisplayName %q, got %q", tt.expected, got)
			}
		})
	}
}
