package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
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
		return &stale, nil
	}

	weatherText := ""
	if len(cond.WeatherDesc) > 0 {
		weatherText = cond.WeatherDesc[0].Value
	}
	s.setCache(ctx, cityID, cityName, cond, raw)

	return &domain.WeatherNow{
		CityName:    cityName,
		Temp:        cond.TempC,
		FeelsLike:   cond.FeelsLikeC,
		WeatherText: weatherText,
		WindDir:     cond.WindDir16,
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
