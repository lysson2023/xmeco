package intelligence

import (
	"context"
	"math"
	"time"
)

// ForecastLoad predicts hourly cooling load for the next 24 hours
// based on current outdoor temperature and typical load profiles.
func (s *Service) ForecastLoad(ctx context.Context, outdoorTemp float64) []LoadForecast {
	now := time.Now()
	currentHour := now.Hour()

	result := make([]LoadForecast, 24)

	// Typical hourly load profile for office buildings (normalized 0-1)
	officeProfile := []float64{
		0.15, 0.10, 0.08, 0.08, 0.10, 0.15, // 0-5
		0.30, 0.55, 0.75, 0.85, 0.92, 0.95, // 6-11
		1.00, 0.98, 0.95, 0.90, 0.85, 0.75, // 12-17
		0.60, 0.40, 0.25, 0.20, 0.18, 0.15, // 18-23
	}

	// Temperature influence: cooling demand rises ~8% per °C above 22°C
	tempInfluence := 1.0
	if outdoorTemp > 22 {
		tempInfluence = 1.0 + (outdoorTemp-22)*0.08
	}
	if outdoorTemp < 18 {
		tempInfluence = 0.6
	}

	// Base cooling load estimate (from device ratings)
	baseLoadKW := estimateSystemLoad()

	for h := range 24 {
		hour := (currentHour + h) % 24
		profile := officeProfile[hour]

		// Temperature varies through the day: sinusoidal
		tempVariation := outdoorTemp + 5*math.Sin(2*math.Pi*float64(h-14)/24)

		loadKW := baseLoadKW * profile * tempInfluence
		if outdoorTemp > 30 {
			loadKW *= 1.15 // extra penalty for hot days
		}

		result[h] = LoadForecast{
			Hour:    hour,
			Temp:    round2(tempVariation),
			LoadKW:  round2(loadKW),
			LoadPct: round2(profile * 100 * tempInfluence),
		}
	}

	return result
}

// estimateSystemLoad returns the base cooling load estimate (kW).
// TODO: replace with actual query summing rated power of chiller devices
// from device_properties divided by ~3 for cooling load conversion.
func estimateSystemLoad() float64 {
	return 200.0
}
