package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"xmeco/internal/domain"
)

const (
	wttrNowURL    = "http://wttr.in/%s?format=j1"
	cacheDuration = 60 * time.Minute
	httpTimeout   = 10 * time.Second
)

// Service 天气服务 — wttr.in (免费无需 Key) + DB 缓存
type Service struct {
	pool   *pgxpool.Pool
	client *http.Client
}

// New creates a new weather service.
func New(pool *pgxpool.Pool) *Service {
	return &Service{
		pool:   pool,
		client: &http.Client{Timeout: httpTimeout},
	}
}

// ---- wttr.in response types ----

type wttrResp struct {
	CurrentCondition []wttrCond `json:"current_condition"`
}

type wttrCond struct {
	TempC       string     `json:"temp_C"`
	FeelsLikeC  string     `json:"FeelsLikeC"`
	Humidity    string     `json:"humidity"`
	Pressure    string     `json:"pressure"`
	WindDir16   string     `json:"winddir16Point"`
	WindSpeedKm string     `json:"windspeedKmph"`
	PrecipMM    string     `json:"precipMM"`
	WeatherCode string     `json:"weatherCode"`
	WeatherDesc []wttrDesc `json:"weatherDesc"`
}

type wttrDesc struct {
	Value string `json:"value"`
}

// ---- Existing cities still use location_id for caching ----

func (s *Service) getCached(ctx context.Context, locationID string) (*domain.WeatherNow, bool) {
	var w domain.WeatherNow
	var fetchedAt time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT city_name, temp, feels_like, icon, weather_text, wind_dir, wind_scale, humidity, precip, pressure, fetched_at
		 FROM weather_cache WHERE location_id=$1 AND expires_at > NOW()
		 ORDER BY fetched_at DESC LIMIT 1`, locationID).
		Scan(&w.CityName, &w.Temp, &w.FeelsLike, &w.Icon, &w.WeatherText,
			&w.WindDir, &w.WindScale, &w.Humidity, &w.Precip, &w.Pressure, &fetchedAt)
	if err != nil {
		return nil, false
	}
	w.FetchedAt = fetchedAt.Format(time.RFC3339)
	w.WeatherText = translateWeather(w.WeatherText, "")
	w.WindDir = translateWindDir(w.WindDir)
	return &w, true
}

func (s *Service) setCache(ctx context.Context, cityID int, cityName string, c *wttrCond, rawJSON string) {
	if cityID <= 0 {
		// No valid city ID; skip cache (will still return live data)
		return
	}
	expiresAt := time.Now().Add(cacheDuration)
	weatherText := ""
	if len(c.WeatherDesc) > 0 {
		weatherText = c.WeatherDesc[0].Value
	}
	locationID := cityName // use city name as cache key
	_, err := s.pool.Exec(ctx,
		`INSERT INTO weather_cache (city_id, location_id, city_name, temp, feels_like, icon, weather_text,
		 wind_dir, wind_scale, humidity, precip, pressure, raw_json, fetched_at, expires_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NOW(),$14)`,
		cityID, locationID, cityName, c.TempC, c.FeelsLikeC, "", weatherText,
		c.WindDir16, c.WindSpeedKm, c.Humidity, c.PrecipMM, c.Pressure, rawJSON, expiresAt)
	if err != nil {
		slog.Warn("weather cache insert failed", "err", err)
	}
}

// fetchWttr calls wttr.in for a city name and returns the current condition.
func (s *Service) fetchWttr(ctx context.Context, cityName string) (*wttrCond, string, error) {
	reqURL := fmt.Sprintf(wttrNowURL, url.QueryEscape(cityName))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "XMECO/1.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("wttr.in request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	var result wttrResp
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, "", fmt.Errorf("wttr.in parse failed: %w", err)
	}
	if len(result.CurrentCondition) == 0 {
		return nil, "", fmt.Errorf("wttr.in returned no data for city: %s", cityName)
	}
	return &result.CurrentCondition[0], string(body), nil
}

// ---- Public methods ----

// GetNow returns current weather for a city by ID.
func (s *Service) GetNow(ctx context.Context, cityID int) (*domain.WeatherNow, error) {
	var cityName string
	err := s.pool.QueryRow(ctx, `SELECT name FROM city WHERE id=$1`, cityID).Scan(&cityName)
	if err != nil {
		return nil, fmt.Errorf("city not found: %w", err)
	}
	return s.getNowByName(ctx, cityID, cityName)
}

// GetNowByCityName returns weather for a city name directly.
func (s *Service) GetNowByCityName(ctx context.Context, cityName string) (*domain.WeatherNow, error) {
	// Try cache first
	if cached, ok := s.getCached(ctx, cityName); ok {
		return cached, nil
	}
	// Resolve city ID from DB for the FK on weather_cache
	var cityID int
	if err := s.pool.QueryRow(ctx, `SELECT id FROM city WHERE name=$1`, cityName).Scan(&cityID); err != nil {
		// City not in DB — use 0 (will fail FK on cache insert but weather data still returned)
		cityID = 0
	}
	return s.getNowByName(ctx, cityID, cityName)
}

func (s *Service) getNowByName(ctx context.Context, cityID int, cityName string) (*domain.WeatherNow, error) {
	// Try cache first
	if cached, ok := s.getCached(ctx, cityName); ok {
		return cached, nil
	}

	// Fetch from wttr.in
	cond, raw, err := s.fetchWttr(ctx, cityName)
	if err != nil {
		// On error, try stale cache
		var stale domain.WeatherNow
		var fetchedAt time.Time
		err2 := s.pool.QueryRow(ctx,
			`SELECT city_name, temp, feels_like, icon, weather_text, wind_dir, wind_scale, humidity, precip, pressure, fetched_at
			 FROM weather_cache WHERE location_id=$1 ORDER BY fetched_at DESC LIMIT 1`, cityName).
			Scan(&stale.CityName, &stale.Temp, &stale.FeelsLike, &stale.Icon, &stale.WeatherText,
				&stale.WindDir, &stale.WindScale, &stale.Humidity, &stale.Precip, &stale.Pressure, &fetchedAt)
		if err2 != nil {
			return nil, err
		}
		stale.FetchedAt = fetchedAt.Format(time.RFC3339)
		stale.WeatherText = translateWeather(stale.WeatherText, "")
		stale.WindDir = translateWindDir(stale.WindDir)
		return &stale, nil
	}

	weatherText := ""
	if len(cond.WeatherDesc) > 0 {
		weatherText = translateWeather(cond.WeatherDesc[0].Value, cond.WeatherCode)
	}
	s.setCache(ctx, cityID, cityName, cond, raw)

	return &domain.WeatherNow{
		CityName:    cityName,
		Temp:        cond.TempC,
		FeelsLike:   cond.FeelsLikeC,
		WeatherText: weatherText,
		WindDir:     translateWindDir(cond.WindDir16),
		WindScale:   cond.WindSpeedKm,
		Humidity:    cond.Humidity,
		Precip:      cond.PrecipMM,
		Pressure:    cond.Pressure,
		FetchedAt:   time.Now().Format(time.RFC3339),
	}, nil
}

// GetWeather is the unified handler: prefers cityID, falls back to city name.
func (s *Service) GetWeather(ctx context.Context, cityID int, cityName string) (*domain.WeatherNow, error) {
	if cityID > 0 {
		return s.GetNow(ctx, cityID)
	}
	if cityName != "" {
		return s.GetNowByCityName(ctx, cityName)
	}
	return nil, fmt.Errorf("either city_id or city_name required")
}

// GetProjectWeather fetches weather for a project's city.
func (s *Service) GetProjectWeather(ctx context.Context, projectID int) (*domain.WeatherNow, error) {
	var cityID *int
	var cityName *string
	err := s.pool.QueryRow(ctx,
		`SELECT city_id, city_name FROM project WHERE id=$1`, projectID).Scan(&cityID, &cityName)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}
	if cityID == nil || *cityID == 0 {
		if cityName != nil && *cityName != "" {
			return s.GetNowByCityName(ctx, *cityName)
		}
		return nil, fmt.Errorf("project has no city configured")
	}
	return s.GetNow(ctx, *cityID)
}

// GetCity returns a single city by ID.
func (s *Service) GetCity(ctx context.Context, id int) (*domain.City, error) {
	var c domain.City
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, COALESCE(province,''), COALESCE(admin_code,''), location_id, lat, lon, sort_order FROM city WHERE id=$1`, id).
		Scan(&c.ID, &c.Name, &c.Province, &c.AdminCode, &c.LocationID, &c.Lat, &c.Lon, &c.SortOrder)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListProvinceCities returns all cities grouped by province (for cascading selector).
func (s *Service) ListProvinceCities(ctx context.Context) ([]domain.ProvinceCities, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT province, id, name, COALESCE(admin_code,''), location_id, lat, lon, sort_order
		 FROM city ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	provMap := make(map[string][]domain.City)
	provOrder := make([]string, 0)

	for rows.Next() {
		var c domain.City
		var prov string
		if err := rows.Scan(&prov, &c.ID, &c.Name, &c.AdminCode, &c.LocationID, &c.Lat, &c.Lon, &c.SortOrder); err != nil {
			slog.Warn("ListProvinceCities scan failed", "err", err)
			continue
		}
		c.Province = prov
		if _, ok := provMap[prov]; !ok {
			provOrder = append(provOrder, prov)
		}
		provMap[prov] = append(provMap[prov], c)
	}

	result := make([]domain.ProvinceCities, 0, len(provOrder))
	for _, prov := range provOrder {
		result = append(result, domain.ProvinceCities{Province: prov, Cities: provMap[prov]})
	}
	return result, nil
}

// SearchCities supports keyword search in local city table.
func (s *Service) SearchCities(ctx context.Context, q string) ([]domain.City, error) {
	query := `SELECT id, name, COALESCE(province,''), COALESCE(admin_code,''), location_id, lat, lon, sort_order FROM city`
	var rows pgx.Rows
	var err error
	if q != "" {
		query += ` WHERE name ILIKE $1 OR province ILIKE $1 ORDER BY sort_order LIMIT 50`
		rows, err = s.pool.Query(ctx, query, "%"+q+"%")
	} else {
		query += ` ORDER BY sort_order LIMIT 50`
		rows, err = s.pool.Query(ctx, query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cities []domain.City
	for rows.Next() {
		var c domain.City
		if err := rows.Scan(&c.ID, &c.Name, &c.Province, &c.AdminCode, &c.LocationID, &c.Lat, &c.Lon, &c.SortOrder); err != nil {
			slog.Warn("SearchCities scan failed", "err", err)
			continue
		}
		cities = append(cities, c)
	}
	return cities, rows.Err()
}

var weatherCN = map[string]string{
	// 晴/多云
	"Sunny": "晴", "Clear": "晴",
	"Partly cloudy": "多云", "Partly Cloudy": "多云",
	"Cloudy": "阴", "Overcast": "阴",
	// 雾/霾
	"Mist": "雾", "Fog": "雾", "Freezing fog": "冻雾",
	"Haze": "霾",
	// 雨
	"Light drizzle": "毛毛雨", "Patchy light drizzle": "局部毛毛雨",
	"Light rain": "小雨", "Light Rain": "小雨",
	"Patchy light rain": "局部小雨",
	"Moderate rain": "中雨", "Moderate Rain": "中雨",
	"Moderate rain at times": "间歇中雨",
	"Heavy rain": "大雨", "Heavy Rain": "大雨",
	"Heavy rain at times": "间歇大雨",
	"Torrential rain shower": "暴雨",
	"Patchy rain nearby": "局部阵雨", "Patchy rain possible": "可能有雨",
	"Light rain shower": "小阵雨", "Light Rain Shower": "小阵雨",
	"Moderate or heavy rain shower": "大阵雨",
	// 雪
	"Light snow": "小雪", "Patchy light snow": "局部小雪",
	"Moderate snow": "中雪",
	"Heavy snow": "大雪", "Patchy heavy snow": "局部大雪",
	"Blizzard": "暴风雪", "Blowing snow": "吹雪",
	"Light snow showers": "小阵雪", "Moderate or heavy snow showers": "大阵雪",
	// 雨夹雪
	"Light sleet": "小雨夹雪", "Moderate or heavy sleet": "雨夹雪",
	"Patchy sleet nearby": "局部雨夹雪",
	"Light sleet showers": "小阵雨夹雪",
	// 冰雹
	"Ice pellets": "冰粒", "Light showers of ice pellets": "小冰粒阵",
	"Moderate or heavy showers of ice pellets": "大冰粒阵",
	// 雷暴
	"Thunderstorm": "雷暴", "Thundery outbreaks possible": "可能有雷暴",
	"Patchy light rain with thunder": "局部雷阵雨",
	"Moderate or heavy rain with thunder": "大雷雨",
	// 风
	"Windy": "大风",
}

// windDirCN maps 16-point compass directions to Chinese.
var windDirCN = map[string]string{
	"N": "北", "NNE": "北东北", "NE": "东北", "ENE": "东东北",
	"E": "东", "ESE": "东东南", "SE": "东南", "SSE": "南东南",
	"S": "南", "SSW": "南西南", "SW": "西南", "WSW": "西西南",
	"W": "西", "WNW": "西西北", "NW": "西北", "NNW": "北西北",
}

func translateWindDir(dir string) string {
	if cn, ok := windDirCN[dir]; ok {
		return cn
	}
	return dir
}

// weatherCodeCN maps WorldWeatherOnline codes to Chinese.
var weatherCodeCN = map[string]string{
	"113": "晴", "116": "多云", "119": "阴", "122": "阴",
	"143": "雾", "248": "雾", "260": "冻雾",
	"176": "小雨", "263": "小雨", "266": "毛毛雨", "293": "毛毛雨", "296": "毛毛雨", "299": "中雨",
	"302": "中雨", "305": "大雨", "308": "大雨", "311": "冻雨", "314": "冻雨",
	"179": "阵雪", "227": "吹雪", "230": "暴风雪",
	"182": "雨夹雪", "185": "冻雨", "281": "冻雨", "284": "冻雨",
	"200": "雷暴", "386": "雷阵雨", "389": "大雷雨",
	"317": "雨夹雪", "320": "雨夹雪", "323": "小雪", "326": "小雪", "329": "中雪",
	"332": "中雪", "335": "大雪", "338": "大雪", "350": "冰粒", "353": "阵雨",
	"356": "中雨", "359": "大雨", "362": "雨夹雪", "365": "雨夹雪",
	"368": "小雪", "371": "大雪", "374": "冰粒", "377": "冰粒",
	"392": "雷阵雪", "395": "大雷雪",
}

func translateWeather(text, code string) string {
	s := strings.TrimSpace(text)
	if cn, ok := weatherCN[s]; ok {
		return cn
	}
	if cn, ok := weatherCodeCN[code]; ok {
		return cn
	}
	if code != "" && len(code) >= 3 {
		switch code[0] {
		case '1': return "晴间多云"
		case '2': return "局部雷暴"
		case '3': return "阵雨"
		case '4': return "雨夹雪"
		}
	}
	return "多云"
}
