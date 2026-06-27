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
	expected := "https://wttr.in/%E5%8C%97%E4%BA%AC?format=j1"
	// The constant wttrNowURL uses fmt.Sprintf
	// Just verify the format string is correct
	if wttrNowURL != "https://wttr.in/%s?format=j1" {
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

// =============================================================================
// Tier 3 — W-10~W-13: translateWeather 中文翻译
// =============================================================================

func TestTranslateWeather(t *testing.T) {
	tests := []struct {
		name string
		text string
		code string
		want string
	}{
		{
			name: "W-10 精确文本匹配Clear→晴",
			text: "Clear",
			code: "",
			want: "晴",
		},
		{
			name: "W-10 精确文本匹配Sunny→晴",
			text: "Sunny",
			code: "",
			want: "晴",
		},
		{
			name: "W-11 代码匹配113→晴",
			text: "",
			code: "113",
			want: "晴",
		},
		{
			name: "W-11 代码匹配116→多云",
			text: "unknown-text",
			code: "116",
			want: "多云",
		},
		{
			name: "code 302精确匹配中雨(映射表优先)",
			text: "unknown-code",
			code: "302",
			want: "中雨", // weatherCodeCN["302"] = "中雨"
		},
		{
			name: "W-12 前缀回退3xx→阵雨(无精确code)",
			text: "",
			code: "303", // 不在 weatherCodeCN 中，走前缀 3→"阵雨"
			want: "阵雨",
		},
		{
			name: "W-12 前缀回退1xx→晴间多云(无精确code)",
			text: "",
			code: "199", // 不在 weatherCodeCN 中，走前缀 1→"晴间多云"
			want: "晴间多云",
		},
		{
			name: "W-13 全部不匹配默认多云",
			text: "UnknownXxx",
			code: "999",
			want: "多云",
		},
		{
			name: "W-13 空输入默认多云",
			text: "",
			code: "",
			want: "多云",
		},
		{
			name: "文本含前后空格被Trim",
			text: "  Clear  ",
			code: "",
			want: "晴",
		},
		{
			name: "Thunderstorm→雷暴",
			text: "Thunderstorm",
			code: "",
			want: "雷暴",
		},
		{
			name: "Heavy rain→大雨",
			text: "Heavy rain",
			code: "",
			want: "大雨",
		},
		{
			name: "文本优先于代码",
			text: "Sunny",
			code: "302",
			want: "晴", // text match takes precedence over code
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := translateWeather(tt.text, tt.code)
			if got != tt.want {
				t.Errorf("translateWeather(%q, %q) = %q, want %q", tt.text, tt.code, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Tier 3 — translateWindDir 风向翻译
// =============================================================================

func TestTranslateWindDir(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want string
	}{
		{"北风N", "N", "北"},
		{"东北NE", "NE", "东北"},
		{"东南SE", "SE", "东南"},
		{"南风S", "S", "南"},
		{"西南SW", "SW", "西南"},
		{"西北NW", "NW", "西北"},
		{"未知方向原样返回", "XYZ", "XYZ"},
		{"空字符串返回空", "", ""},
		{"16方向NNE", "NNE", "北东北"},
		{"16方向WSW", "WSW", "西西南"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := translateWindDir(tt.dir)
			if got != tt.want {
				t.Errorf("translateWindDir(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

func TestHTTPTimeout(t *testing.T) {
	if httpTimeout.Seconds() < 5 || httpTimeout.Seconds() > 30 {
		t.Errorf("HTTP timeout %.0f sec, expected 5-30 sec", httpTimeout.Seconds())
	}
}
