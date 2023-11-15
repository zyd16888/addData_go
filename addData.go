package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
	"time"

	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/spf13/viper"
	"github.com/xuri/excelize/v2"
)

var AppConfig Settings

func init() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal("Error reading config file:", err)
	}

	err := viper.Unmarshal(&AppConfig)

	if err != nil {
		log.Fatal("Error decoding config file:", err)
	}
}

func main() {
	data := readDeviceData(AppConfig.ExcelName, AppConfig.SheetName)
	log.Println(data) // Use the 'data' variable

	tempHumidityData := readTempHumidityData(AppConfig.ExcelName, AppConfig.TempHumiditySheet)
	log.Println(tempHumidityData) // Use the 'tempHumidityData' variable

	go run(data)

	select {}

	// fmt.Println("Hello, World!")
	// log.Println("Hello, World!")
}

func run(data []DataItem) {
	for {
		insertData, updatedData := buildDeviceData(data)
		insertDB(insertData)
		time.Sleep(time.Duration(AppConfig.RunInterval) * time.Second)
		data = updatedData
	}

}

func readDeviceData(fileName, sheetName string) []DataItem {
	f, err := excelize.OpenFile(fileName)
	if err != nil {
		log.Fatal("Error opening Excel file: ", err)

	}

	rows, err := f.GetRows(sheetName)

	if err != nil {
		log.Fatal("Error getting rows: ", err)

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
		log.Fatal("Error opening Excel file:", err)
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		log.Fatal("Error getting rows:", err)
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
	log.Printf("开始构造数据，数据长度:%d\n", len(data))

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
		log.Printf("构造数据: %v\n", itemDict)
	}
	log.Printf("构造数据成功, 数据长度为: %d\n", len(result))
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

func insertDB(data []interface{}) {
	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr: fmt.Sprintf("http://%s:%d", AppConfig.Host, AppConfig.Port),
	})

	if err != nil {
		log.Fatal("Error creating InfluxDB Client: ", err)
	}

	defer c.Close()

	bp, _ := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  AppConfig.Database,
		Precision: "ms",
	})

	for _, item := range data {
		itemDict, ok := item.(map[string]interface{})
		if !ok {
			log.Printf("Invalid data format: %v\n", item)
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
			log.Printf("Error creating InfluxDB point: %v\n", err)
			continue
		}

		// Add the point to the batch
		bp.AddPoint(pt)
	}

	if err := c.Write(bp); err != nil {
		log.Fatal("Error writing to InfluxDB: ", err)
	}

	dataStr, _ := json.Marshal(data)
	log.Printf("插入数据成功, 数据:%s\n", dataStr)
	log.Printf("插入数据成功, 数据长度:%d\n", len(data))
}
