package main

import (
	"addData/logger"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"

	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/patrickmn/go-cache"
	"github.com/spf13/viper"
	"github.com/xuri/excelize/v2"
)

var loger *logger.Logger

var AppConfig Settings

var c *cache.Cache

func init() {

	var err error
	loger, err = logger.NewLogger("info.log", "error.log", "debug.log")
	if err != nil {
		panic(err)
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		loger.Error.Fatal("Error reading config file:", err)
	}

	err = viper.Unmarshal(&AppConfig)

	if err != nil {
		loger.Error.Fatal("Error decoding config file:", err)
	}

	loger.Info.Println("配置文件读取成功")
	jsonStr, _ := json.Marshal(AppConfig)
	loger.Info.Printf("配置文件内容: %s\n", jsonStr)

	c = cache.New(time.Duration(AppConfig.CacheTime)*time.Second, time.Duration(AppConfig.CacheTime+10)*time.Second)

}

func main() {
	data := readDeviceData(AppConfig.ExcelName, AppConfig.SheetName)
	loger.Debug.Println(data) // Use the 'data' variable

	tempHumidityData := readTempHumidityData(AppConfig.ExcelName, AppConfig.TempHumiditySheet)
	loger.Debug.Println(tempHumidityData) // Use the 'tempHumidityData' variable

	loger.Debug.Println(time.Now().Format("2006-01-02 15:04:05"))

	go run(data)
	go runTempHumidity(tempHumidityData)

	<-make(chan struct{})
}

func run(data []DataItem) {
	if AppConfig.RunTimes == 0 {
		for {
			loger.Debug.Println(time.Now().Format("2006-01-02 15:04:05"))
			insertData, updatedData := buildDeviceData(data)
			insertDB(insertData)
			time.Sleep(time.Duration(AppConfig.RunInterval) * time.Second)
			data = updatedData
		}
	} else {
		for i := 0; i < AppConfig.RunTimes; i++ {
			loger.Debug.Println(time.Now().Format("2006-01-02 15:04:05"))
			insertData, updatedData := buildDeviceData(data)
			insertDB(insertData)
			time.Sleep(time.Duration(AppConfig.RunInterval) * time.Second)
			data = updatedData
		}
	}

}

func runTempHumidity(data []TempHumidityItem) {
	if AppConfig.RunTimes == 0 {
		for {
			loger.Debug.Println(time.Now().Format("2006-01-02 15:04:05"))
			insertData := buildTempHumidityData(data)
			insertDB(insertData)
			time.Sleep(time.Duration(AppConfig.RunInterval) * time.Second)
		}
	} else {
		for i := 0; i < AppConfig.RunTimes; i++ {
			loger.Debug.Println(time.Now().Format("2006-01-02 15:04:05"))
			insertData := buildTempHumidityData(data)
			insertDB(insertData)
			time.Sleep(time.Duration(AppConfig.RunInterval) * time.Second)
		}
	}

}

func readDeviceData(fileName, sheetName string) []DataItem {
	f, err := excelize.OpenFile(fileName)
	if err != nil {
		loger.Error.Fatal("Error opening Excel file: ", err)

	}

	rows, err := f.GetRows(sheetName)

	if err != nil {
		loger.Error.Fatal("Error getting rows: ", err)

	}

	var result []DataItem

	for _, row := range rows {
		data, _ := strconv.ParseFloat(row[6], 64)
		item := DataItem{
			DeviceModuleID:   row[0],
			StationID:        row[1],
			StationGatewayID: row[2],
			DeviceID:         row[3],
			DataMin:          row[4],
			DataMax:          row[5],
			Data:             data,
			Status:           row[7],
		}
		result = append(result, item)
	}

	return result

}

func readTempHumidityData(filename, sheetName string) []TempHumidityItem {
	f, err := excelize.OpenFile(filename)
	if err != nil {
		loger.Error.Fatal("Error opening Excel file:", err)
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		loger.Error.Fatal("Error getting rows:", err)
	}

	var result []TempHumidityItem
	for _, row := range rows {
		item := TempHumidityItem{
			DeviceModuleID:   row[0],
			StationID:        row[1],
			StationGatewayID: row[2],
			DeviceID:         row[3],
			Longitude:        row[4],
			Latitude:         row[5],
			Type:             row[6],
		}
		result = append(result, item)
	}
	return result
}

func buildDeviceData(data []DataItem) ([]interface{}, []DataItem) {
	var result []interface{}
	loger.Info.Printf("开始构造设备数据， 从excel获取的数据长度:%d\n", len(data))

	for i, item := range data {
		if i == 0 {
			continue
		}
		itemDict := make(map[string]interface{})
		itemDict["measurement"] = AppConfig.Measurement
		itemDict["tags"] = map[string]string{
			"station_id":         item.StationID,
			"station_gateway_id": item.StationGatewayID,
			"device_id":          item.DeviceID,
			"device_module_id":   item.DeviceModuleID,
		}
		itemDict["time"] = time.Now().UnixNano() / int64(time.Millisecond)

		newData := getData(item.DataMin, item.DataMax, item.Data, item.Status)
		itemDict["fields"] = map[string]interface{}{
			"data":        newData,
			"status":      "1",
			"prefix_data": 0.0,
		}
		// 替换原数据
		data[i].Data = newData

		result = append(result, itemDict)
		loger.Debug.Printf("构造数据: %v\n", itemDict)
	}
	loger.Info.Printf("构造数据成功, 数据长度为: %d\n", len(result))
	return result, data
}

func getData(dataMin string, dataMax string, data float64, status string) float64 {
	dataMaxFloat, _ := strconv.ParseFloat(dataMax, 64)
	dataMinFloat, _ := strconv.ParseFloat(dataMin, 64)

	if status == "1" {
		return data
	} else if status == "2" {
		// 最大值与最小值之间生成随机数
		randomFloat := dataMinFloat + rand.Float64()*(dataMaxFloat-dataMinFloat)
		return math.Round(randomFloat*100) / 100
	}

	// 模拟数据
	var newData float64
	if data == 0 {
		newData = rand.Float64()*(dataMaxFloat-dataMinFloat) + dataMinFloat
	} else {
		src := rand.NewSource(time.Now().UnixNano())
		r := rand.New(src)
		choices := []float64{-0.1, 0, 0.1, -0.05, 0.05}
		choice := choices[r.Intn(len(choices))]
		newData = newData + choice + data
		if newData > dataMaxFloat {
			newData -= 0.1
		} else if newData < dataMinFloat {
			newData += 0.1
		}
	}
	return math.Round(newData*100) / 100
}

func getTempHumidity(lon, lat string) (float64, float64) {

	key := lon + "," + lat

	if x, found := c.Get(key); found {
		return x.(WeatherResponse).Main.Temp, x.(WeatherResponse).Main.Humidity
	}

	baseUrl := "http://api.openweathermap.org/data/2.5/weather"
	params := url.Values{}
	params.Add("lat", lat)
	params.Add("lon", lat)
	params.Add("appid", "18638206978f9d9119ec5ebf472d8a84")
	params.Add("lang", "zh_cn")
	params.Add("units", "Metric")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s?%s", baseUrl, params.Encode()))
	if err != nil {
		loger.Error.Printf("getTempHumidity error: %v", err)
		return 0, 0
	}
	defer resp.Body.Close()

	var weatherResponse WeatherResponse
	err = json.NewDecoder(resp.Body).Decode(&weatherResponse)
	if err != nil {
		loger.Error.Printf("getTempHumidity error: %v", err)
		return 0, 0
	}

	c.Set(key, weatherResponse, cache.DefaultExpiration)

	loger.Info.Printf("获取温湿度数据成功,经度:%v,纬度:%v, 温度:%v,湿度:%v", lon, lat, weatherResponse.Main.Temp, weatherResponse.Main.Humidity)
	return weatherResponse.Main.Temp, weatherResponse.Main.Humidity
}

func buildTempHumidityData(data []TempHumidityItem) []interface{} {

	loger.Info.Printf("开始构造温湿度数据， 从excel获取的数据长度:%d\n", len(data))

	var result []interface{}

	for i, item := range data {
		if i == 0 {
			continue
		}

		itemDict := make(map[string]interface{})
		itemDict["measurement"] = AppConfig.Measurement
		itemDict["tags"] = map[string]string{
			"station_id":         item.StationID,
			"station_gateway_id": item.StationGatewayID,
			"device_id":          item.DeviceID,
			"device_module_id":   item.DeviceModuleID,
		}
		itemDict["time"] = time.Now().UnixNano() / int64(time.Millisecond)

		temp, humidity := getTempHumidity(item.Longitude, item.Latitude)

		if item.Type == "temp" {
			itemDict["fields"] = map[string]interface{}{
				"data":        temp,
				"status":      "1",
				"prefix_data": 0.0,
			}
		}

		if item.Type == "humidity" {
			itemDict["fields"] = map[string]interface{}{
				"data":        humidity,
				"status":      "1",
				"prefix_data": 0.0,
			}
		}
		result = append(result, itemDict)
		loger.Debug.Printf("构造数据: %v\n", itemDict)
	}
	loger.Info.Printf("构造温湿度数据成功, 数据长度为: %d\n", len(result))
	return result
}

func insertDB(data []interface{}) {
	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr: fmt.Sprintf("http://%s:%d", AppConfig.Host, AppConfig.Port),
	})

	if err != nil {
		loger.Error.Fatal("Error creating InfluxDB Client: ", err)
	}

	defer c.Close()

	bp, _ := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  AppConfig.Database,
		Precision: "ms",
	})

	for _, item := range data {
		itemDict, ok := item.(map[string]interface{})
		if !ok {
			loger.Error.Printf("Invalid data format: %v\n", item)
			continue
		}

		// Create a new InfluxDB point
		pt, err := client.NewPoint(
			itemDict["measurement"].(string),
			itemDict["tags"].(map[string]string),
			itemDict["fields"].(map[string]interface{}),
			time.Unix(0, itemDict["time"].(int64)*int64(time.Millisecond)),
		)
		if err != nil {
			loger.Error.Printf("Error creating InfluxDB point: %v\n", err)
			continue
		}

		// Add the point to the batch
		bp.AddPoint(pt)
	}

	if err := c.Write(bp); err != nil {
		loger.Error.Fatal("Error writing to InfluxDB: ", err)
	}

	dataStr, _ := json.Marshal(data)
	loger.Debug.Printf("插入数据成功, 数据:%s\n", dataStr)
	loger.Info.Printf("插入数据成功, 数据长度:%d\n", len(data))
}
