package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

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

type Response struct {
	Hourly HourlyWeather `json:"hourly"`
	City   string
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
	urlEnd := fmt.Sprintf("&forecast_days=%v", forecastDays)
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
	createTable := `
		CREATE TABLE IF NOT EXISTS weather (
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
			winddir_10m INTEGER
		);`
	if _, err := db.Exec(createTable); err != nil {
		log.Fatal(err)
	}
	for index, _ := range response.Hourly.Time {
		insertInto := `
			INSERT INTO weather (
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
				winddir_10m
			)
			VALUES ($1,CURRENT_DATE,$2,$3,$4,$5,$6,$7,$8,$9,$10);`
		if _, err := db.Exec(insertInto,
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
		); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("Data has been successfully written to Database.")
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
	for i, city := range config.Cities {
		res, err := getMeteoData(config.Cities[i].Coordinates, config.Parameters, config.ForecastDays)
		if err != nil {
			log.Fatal(err)
		}
		writeDataToDb(res, db_port, db_host, db_database, db_user, db_pass, city.Name)

	}

}
