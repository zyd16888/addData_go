package main // 替换为你的包名

type Settings struct {
	ExcelName         string `mapstructure:"excel_name"`
	SheetName         string `mapstructure:"sheet_name"`
	TempHumiditySheet string `mapstructure:"temp_humidity_sheet"`
	RunTimes          int    `mapstructure:"run_times"`
	RunInterval       int    `mapstructure:"run_interval"`
	Host              string `mapstructure:"host"`
	Port              int    `mapstructure:"port"`
	Database          string `mapstructure:"database"`
	Measurement       string `mapstructure:"measurement"`
	CacheTime         int    `mapstructure:"cache_time"`
	OpenWeatherMapAPI string `mapstructure:"openweathermap_api"`
}

type DataItem struct {
	DeviceModuleID   string
	StationID        string
	StationGatewayID string
	DeviceID         string
	DataMin          string
	DataMax          string
	Data             float64
	Status           string
}

type TempHumidityItem struct {
	DeviceModuleID   string
	StationID        string
	StationGatewayID string
	DeviceID         string
	Longitude        string
	Latitude         string
	Type             string
}
type WeatherResponse struct {
	Main struct {
		Temp     float64 `json:"temp"`
		Humidity float64 `json:"humidity"`
	} `json:"main"`
}
