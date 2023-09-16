package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Cities []ConfigCity `yaml:"cities"`
}
type ConfigCity struct {
	Name        string    `yaml:"name"`
	Coordinates []float64 `yaml:"coordinates"`
}

type Hours struct {
	Time []string `json:"time"`
}

type Response struct {
	Hourly Hours
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

func getMeteoData(coordinates []float64) {
	url := "https://api.open-meteo.com/v1/forecast?latitude=%v&longitude=%v&hourly=temperature_2m,precipitation_probability,precipitation,rain,snowfall,cloudcover,windspeed_10m,winddirection_10m&forecast_days=16"
	uniqueUrl := fmt.Sprintf(url, coordinates[0], coordinates[1])
	res, err := http.Get(uniqueUrl)
	if err != nil {
		log.Fatal(err)
	}
	responseData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	var responseObject Response
	json.Unmarshal(responseData, &responseObject)
	fmt.Printf("%v", responseObject)
}

func main() {
	config, err := readConfig("config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	getMeteoData(config.Cities[5].Coordinates)

}
