package types

type Config struct {
	Cities       []ConfigCity `yaml:"cities"`
	Parameters   []string     `yaml:"parameters"`
	ForecastDays int8         `yaml:"forecast_days"`
}
type ConfigCity struct {
	Name        string    `yaml:"name"`
	Coordinates []float64 `yaml:"coordinates"`
	Email       bool      `yaml:"email"`
}

type EmailData struct {
	Temperature   map[int]float32
	Precipitation map[int][]float32
	WeatherCode   map[int]string
	Sunrise       string
	Sunset        string
}

type Response struct {
	Hourly HourlyWeather `json:"hourly"`
	Daily  DailyWeather  `json:"daily"`
	City   string
}
type DailyWeather struct {
	Time    []string `json:"time"`
	Sunrise []string `json:"sunrise"`
	Sunset  []string `json:"sunset"`
}
type HourlyWeather struct {
	Time          []string  `json:"time"`
	Temp_2m       []float32 `json:"temperature_2m"`
	PrecProb      []int8    `json:"precipitation_probability"`
	Prec          []float32 `json:"precipitation"`
	Rain          []float32 `json:"rain"`
	Snow          []float32 `json:"snowfall"`
	CloudCover    []int8    `json:"cloudcover"`
	Windspeed_10m []float32 `json:"windspeed_10m"`
	Winddir_10m   []int8    `json:"winddirection_10m"`
	WeatherCode   []int8    `json:"weathercode"`
}
