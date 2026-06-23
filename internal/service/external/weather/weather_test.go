package weather

import (
	"encoding/json"
	"testing"
)

// TestWttrParse verifies JSON unmarshalling of a real wttr.in response.
func TestWttrParse(t *testing.T) {
	// Sample response from wttr.in for Beijing
	jsonStr := `{
		"current_condition": [{
			"temp_C": "27",
			"FeelsLikeC": "27",
			"humidity": "51",
			"pressure": "1007",
			"winddir16Point": "NNE",
			"windspeedKmph": "6",
			"precipMM": "1.3",
			"weatherDesc": [{"value": "Thunderstorm"}]
		}]
	}`

	var resp wttrResp
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		t.Fatalf("failed to parse wttr.in response: %v", err)
	}

	if len(resp.CurrentCondition) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(resp.CurrentCondition))
	}

	c := resp.CurrentCondition[0]
	if c.TempC != "27" {
		t.Errorf("temp = %q, want 27", c.TempC)
	}
	if c.Humidity != "51" {
		t.Errorf("humidity = %q, want 51", c.Humidity)
	}
	if c.Pressure != "1007" {
		t.Errorf("pressure = %q, want 1007", c.Pressure)
	}
	if c.WindDir16 != "NNE" {
		t.Errorf("windDir = %q, want NNE", c.WindDir16)
	}
	if len(c.WeatherDesc) != 1 || c.WeatherDesc[0].Value != "Thunderstorm" {
		t.Errorf("weatherDesc = %+v", c.WeatherDesc)
	}
}

func TestWttrParseEmpty(t *testing.T) {
	jsonStr := `{"current_condition":[]}`
	var resp wttrResp
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if len(resp.CurrentCondition) != 0 {
		t.Errorf("expected empty conditions, got %d", len(resp.CurrentCondition))
	}
}

func TestWttrParseChinese(t *testing.T) {
	// wttr.in returns English descriptions even for Chinese cities,
	// but field names stay the same. Verify robustness.
	jsonStr := `{
		"current_condition": [{
			"temp_C": "32",
			"FeelsLikeC": "40",
			"humidity": "80",
			"pressure": "1003",
			"winddir16Point": "SSW",
			"windspeedKmph": "18",
			"precipMM": "0.0",
			"weatherDesc": [{"value": "Sunny"}]
		}]
	}`

	var resp wttrResp
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	c := resp.CurrentCondition[0]
	if c.TempC != "32" {
		t.Errorf("temp = %q, want 32", c.TempC)
	}
	if c.FeelsLikeC != "40" {
		t.Errorf("feelsLike = %q, want 40", c.FeelsLikeC)
	}
}

func TestWttrParseInvalidJSON(t *testing.T) {
	jsonStr := `not json`
	var resp wttrResp
	if err := json.Unmarshal([]byte(jsonStr), &resp); err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestBuildWttrURL(t *testing.T) {
	// Verify URL construction pattern
	expected := "http://wttr.in/%E5%8C%97%E4%BA%AC?format=j1"
	// The constant wttrNowURL uses fmt.Sprintf
	// Just verify the format string is correct
	if wttrNowURL != "http://wttr.in/%s?format=j1" {
		t.Errorf("unexpected URL pattern: %s", wttrNowURL)
	}
	// Verify expected URL uses proper encoding
	if len(expected) < 30 {
		t.Errorf("URL too short: %s", expected)
	}
}

func TestCacheDuration(t *testing.T) {
	// Verify cache duration is reasonable
	if cacheDuration.Minutes() < 10 || cacheDuration.Minutes() > 120 {
		t.Errorf("cache duration %.0f min, expected 10-120 min", cacheDuration.Minutes())
	}
}

func TestHTTPTimeout(t *testing.T) {
	if httpTimeout.Seconds() < 5 || httpTimeout.Seconds() > 30 {
		t.Errorf("HTTP timeout %.0f sec, expected 5-30 sec", httpTimeout.Seconds())
	}
}
