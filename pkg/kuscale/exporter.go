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
package kuscale

import (
	"log"
	"github.com/prometheus/client_golang/prometheus"
)


type Exporter struct {
	Limit				 			*prometheus.CounterVec
	Usage 					 		*prometheus.CounterVec
	AvgUsage 				 		*prometheus.CounterVec
	AvgAvgUsage 				 	*prometheus.CounterVec
	UpdateCount					 	*prometheus.GaugeVec
}	

type ExporterCollector struct {
	Exporter *Exporter
}

func (ec ExporterCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(ec, ch)
}

func (ec ExporterCollector) collect() error {
	for _, pod := range LivePodMap {
		name := pod.containerName
		id := pod.podName
		node := config.NodeName

		for rn, ri := range pod.CI.RIs {
			ec.Exporter.Limit.WithLabelValues(append([]string{rn, id, node})...).Add(ri.Limit())
			ec.Exporter.Usage.WithLabelValues(append([]string{rn, id, node})...).Add(ri.Usage())
			ec.Exporter.AvgUsage.WithLabelValues(append([]string{rn, id, node})...).Add(ri.AvgUsage())
			ec.Exporter.AvgAvgUsage.WithLabelValues(append([]string{rn, id, node})...).Add(ri.AvgAvgUsage())
		}
		
		ec.Exporter.UpdateCount.WithLabelValues(append([]string{name, id, node})...).Add(float64(pod.CI.UpdateCount))	
	}
	return nil
}


func (ec ExporterCollector) Collect(ch chan<- prometheus.Metric) {
	ec.Exporter.Limit.Reset()
	ec.Exporter.Usage.Reset()
	ec.Exporter.AvgUsage.Reset()
	ec.Exporter.AvgAvgUsage.Reset()
	ec.Exporter.UpdateCount.Reset()
	
	if err := ec.collect(); err != nil {
		log.Printf("Error reading container stats: %s", err)
	} 
	
	ec.Exporter.Limit.Collect(ch)
	ec.Exporter.Usage.Collect(ch)
	ec.Exporter.AvgUsage.Collect(ch)
	ec.Exporter.AvgAvgUsage.Collect(ch)
	ec.Exporter.UpdateCount.Collect(ch)	
}


func NewExporter(reg prometheus.Registerer) *Exporter {
	dm := &Exporter{
		Limit: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:      "Limit",
			Help:      "Resource Limit",
		},
			append([]string{"name", "id", "node"}),
		),
		Usage: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:      "Usage",
			Help:      "Resource Usage",
		},
			append([]string{"name", "id", "node"}),
		),
		AvgUsage: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:      "AvgUsage",
			Help:      "Resource AvgUsage",
		},
			append([]string{"name", "id", "node"}),
		),
		AvgAvgUsage: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:      "AvgAvgUsage",
			Help:      "Resource AvgAvgUsage",
		},
			append([]string{"name", "id", "node"}),
		),	
		UpdateCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "UpdateCount",
			Help:      "UpdateCount",
		},
			append([]string{"name", "id", "node"}),
		),
	}
	ec := ExporterCollector{Exporter: dm}

	reg.MustRegister(ec)
	return dm
}