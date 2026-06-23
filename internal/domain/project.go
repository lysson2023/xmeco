package domain

import "time"

type Project struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	AgentID   *int      `json:"agent_id"`
	Address   string    `json:"address"`
	AdminCode string    `json:"admin_code"`
	CityID    *int      `json:"city_id"`
	CityName  string    `json:"city_name"`
	CreatedAt time.Time `json:"created_at"`
}

// City 城市
type City struct {
	ID         int      `json:"id"`
	Name       string   `json:"name"`
	Province   string   `json:"province"`
	AdminCode  string   `json:"admin_code"`
	LocationID string   `json:"location_id"`
	Lat        *float64 `json:"lat"`
	Lon        *float64 `json:"lon"`
	SortOrder  int      `json:"sort_order"`
}

// ProvinceCities 省→市树形结构（用于级联选择器）
type ProvinceCities struct {
	Province string `json:"province"`
	Cities   []City `json:"cities"`
}

// WeatherNow 实时天气
type WeatherNow struct {
	CityName    string `json:"city_name"`
	Temp        string `json:"temp"`
	FeelsLike   string `json:"feels_like"`
	Icon        string `json:"icon"`
	WeatherText string `json:"weather_text"`
	WindDir     string `json:"wind_dir"`
	WindScale   string `json:"wind_scale"`
	Humidity    string `json:"humidity"`
	Precip      string `json:"precip"`
	Pressure    string `json:"pressure"`
	FetchedAt   string `json:"fetched_at"`
}