// Copyright 2022 Hyeon-Jun Jang, SSLab, Konkuk University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"os"
	"log"
	"gopkg.in/yaml.v2"
)

	
type Configuraion struct {
	MonitoringPeriod 	int			`yaml:"monitoringperiodsec"`
	WindowSize			int			`yaml:"windowsize"`
	NodeName			string
	MonitoringMode			bool
}

func LoadConfig(config *Configuraion) () {

	file, err := os.Open("./config.yaml")
	defer file.Close()
	if err != nil {
		log.Fatal(err)
	}
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		log.Fatal(err)
	}	
}

func PrintConfig(config *Configuraion) () {
	log.Println("MonitoringPeriod : ", config.MonitoringPeriod)
	log.Println("WindowSize : ", config.WindowSize)
	log.Println("NodeName : ", config.NodeName)
	log.Println("MonitoringMode : ", config.MonitoringMode)
}