package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"sort"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Cities       []ConfigCity `yaml:"cities"`
	Parameters   []string     `yaml:"parameters"`
	ForecastDays int8         `yaml:"forecast_days"`
}
type ConfigCity struct {
	Name        string    `yaml:"name"`
	Coordinates []float64 `yaml:"coordinates"`
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

func readConfig(file string) (Config, error) {
	f, err := os.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}
	var config Config
	err = yaml.Unmarshal(f, &config)
	if err != nil {
		log.Fatal(err)
	}
	return config, err
}

func getMeteoData(coordinates []float64, parameters []string, forecastDays int8) (Response, error) {
	urlStart := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%v&longitude=%v&hourly=", coordinates[0], coordinates[1])
	urlEnd := fmt.Sprintf("&daily=sunrise,sunset&timezone=auto&forecast_days=%v", forecastDays)
	urlMiddlePart := ""
	for index, p := range parameters {
		if index < len(parameters)-1 {
			urlMiddlePart = urlMiddlePart + p + ","
		} else if index == len(parameters)-1 {
			urlMiddlePart = urlMiddlePart + p
		}
	}
	url := urlStart + urlMiddlePart + urlEnd

	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	responseData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	var responseObject Response
	json.Unmarshal(responseData, &responseObject)
	return responseObject, err
}

func writeDataToDb(response Response, pgPort, pgHost, pgDatabase, pgUser, pgPass, city string) {
	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", pgHost, pgPort, pgUser, pgPass, pgDatabase)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}
	createTableHourly := `
		CREATE TABLE IF NOT EXISTS weather_hourly (
			id SERIAL PRIMARY KEY, 
			city VARCHAR(50),
			reg_date DATE,
			date TIMESTAMP, 
			temp_2m DECIMAL, 
			prec_prob INTEGER, 
			prec DECIMAL, 
			rain DECIMAL, 
			snow DECIMAL, 
			cloud_cover INTEGER, 
			windspeed_10m DECIMAL, 
			winddir_10m INTEGER,
			weather_code INTEGER
		);`
	if _, err := db.Exec(createTableHourly); err != nil {
		log.Fatal(err)
	}
	createTableDaily := `
		CREATE TABLE IF NOT EXISTS weather_daily (
			id SERIAL PRIMARY KEY,
			city VARCHAR(50),
			reg_date DATE,
			date DATE,
			sunrise TIMESTAMP,
			sunset TIMESTAMP
		);`
	if _, err := db.Exec(createTableDaily); err != nil {
		log.Fatal(err)
	}
	for index, _ := range response.Hourly.Time {
		insertIntoHourly := `
			INSERT INTO weather_hourly (
				city,
				reg_date,
				date,
				temp_2m,
				prec_prob,
				prec,
				rain,
				snow,
				cloud_cover,
				windspeed_10m,
				winddir_10m,
				weather_code
			)
			VALUES ($1,CURRENT_DATE,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11);`
		if _, err := db.Exec(insertIntoHourly,
			city,
			response.Hourly.Time[index],
			response.Hourly.Temp_2m[index],
			response.Hourly.PrecProb[index],
			response.Hourly.Prec[index],
			response.Hourly.Rain[index],
			response.Hourly.Snow[index],
			response.Hourly.CloudCover[index],
			response.Hourly.Windspeed_10m[index],
			response.Hourly.Winddir_10m[index],
			response.Hourly.WeatherCode[index],
		); err != nil {
			log.Fatal(err)
		}
	}
	for index, _ := range response.Daily.Time {
		insertIntoDaily := `
			INSERT INTO weather_daily (
				city,
				reg_date,
				date,
				sunrise,
				sunset
			)
			VALUES ($1,CURRENT_DATE,$2,$3,$4);`
		if _, err := db.Exec(insertIntoDaily,
			city,
			response.Daily.Time[index],
			response.Daily.Sunrise[index],
			response.Daily.Sunset[index],
		); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("Data has been successfully written to Database.")
}

func sortedKeys[V any](m map[int]V) []int {
	keys := make([]int, 0)
	for k, _ := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func createEmailData(response Response) EmailData {
	var emailData EmailData
	emailData.Temperature = make(map[int]float32)
	emailData.Precipitation = make(map[int][]float32)
	emailData.WeatherCode = make(map[int]string)
	for index, _ := range response.Hourly.Time[:24] {
		actualTime, err := time.Parse("2006-01-02T15:04", response.Hourly.Time[index])
		if err != nil {
			panic(err)
		}
		actualHour := actualTime.Hour()
		emailData.Temperature[actualHour] = response.Hourly.Temp_2m[index]
		if response.Hourly.Prec[index] > 0 {
			emailData.Precipitation[actualHour] = append(emailData.Precipitation[actualHour], response.Hourly.Prec[index])
			emailData.Precipitation[actualHour] = append(emailData.Precipitation[actualHour], float32(response.Hourly.PrecProb[index]))
		}
		if response.Hourly.WeatherCode[index] == 95 || response.Hourly.WeatherCode[index] == 96 || response.Hourly.WeatherCode[index] == 99 {
			emailData.WeatherCode[actualHour] = "VIHAR VÁRHATÓ!"
		}
	}
	return emailData
}

func writeEmail(emailData EmailData, city, user, sender, pass, receiver, host, port string) {
	toAddresses := []string{sender}
	hostAndPort := fmt.Sprintf("%s"+":"+"%s", host, port)
	tempString := ""
	precString := ""
	weatherCodeString := ""
	sortedTemp := sortedKeys(emailData.Temperature)
	sortedPrec := sortedKeys(emailData.Precipitation)
	sortedWeatherCode := sortedKeys(emailData.WeatherCode)
	for indexTemp, _ := range sortedTemp {
		tempString = tempString + fmt.Sprintf("%v ora - %v fok\n", indexTemp, emailData.Temperature[indexTemp])
	}
	for indexPrec, _ := range sortedPrec {
		precString = precString + fmt.Sprintf("%v ora - %v mm - %v valoszinuseg\n", indexPrec, emailData.Precipitation[indexPrec][0], emailData.Precipitation[indexPrec][1])
	}
	for indexWeatherCode, _ := range sortedWeatherCode {
		weatherCodeString = weatherCodeString + fmt.Sprintf("%v ora - %v\n", indexWeatherCode, emailData.WeatherCode[indexWeatherCode])
	}
	msgString := fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: Mai idojaras - %s\r\n\r\n"+
			"Homerseklet:\n"+
			"%s\n"+
			"%s\n"+
			"%s\n"+
			"\r\n",
		sender,
		sender,
		city,
		tempString,
		precString,
		weatherCodeString,
	)
	msg := []byte(msgString)
	auth := smtp.PlainAuth("", user, pass, host)
	err := smtp.SendMail(hostAndPort, auth, sender, toAddresses, msg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Email sent successfully.")
}

func main() {
	config, err := readConfig("config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	err2 := godotenv.Load(".env.secret")
	if err2 != nil {
		log.Fatal()
	}
	db_user := os.Getenv("POSTGRES_USER")
	db_pass := os.Getenv("POSTGRES_PASS")
	db_host := os.Getenv("POSTGRES_HOST")
	db_port := os.Getenv("POSTGRES_PORT")
	db_database := os.Getenv("POSTGRES_DB")
	email_user := os.Getenv("EMAIL_USER")
	email_sender := os.Getenv("EMAIL_SENDER")
	email_sender_pass := os.Getenv("EMAIL_SENDER_PASS")
	receiver := os.Getenv("RECEIVER")
	smtp_host := os.Getenv("SMTP_HOST")
	smtp_port := os.Getenv("SMTP_PORT")

	for i, city := range config.Cities {
		res, err := getMeteoData(config.Cities[i].Coordinates, config.Parameters, config.ForecastDays)
		if err != nil {
			log.Fatal(err)
		}
		writeDataToDb(res, db_port, db_host, db_database, db_user, db_pass, city.Name)
		if city.Name == "Sopron" || city.Name == "Ravazd" {
			emailData := createEmailData(res)
			writeEmail(emailData, city.Name, email_user, email_sender, email_sender_pass, receiver, smtp_host, smtp_port)
		}
	}

}
