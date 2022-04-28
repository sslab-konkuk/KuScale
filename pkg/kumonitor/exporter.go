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
package kumonitor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"k8s.io/klog"
)


type Exporter struct {
	Limit				 			*prometheus.CounterVec
	Usage 					 		*prometheus.CounterVec
	AvgUsage 				 		*prometheus.CounterVec
	AvgAvgUsage 				 	*prometheus.CounterVec
	UpdateCount					 	*prometheus.GaugeVec
}	

type ExporterCollector struct {
	exporter 	*Exporter
	monitor 	*Monitor
}

func (ec ExporterCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(ec, ch)
}

func (ec ExporterCollector) collect() error {
	for _, pod := range ec.monitor.livePodMap {
		name := pod.containerName
		id := pod.podName
		node := ec.monitor.config.nodeName

		for rn, ri := range pod.CI.RIs {
			ec.exporter.Limit.WithLabelValues(append([]string{rn, id, node})...).Add(ri.Limit())
			ec.exporter.Usage.WithLabelValues(append([]string{rn, id, node})...).Add(ri.Usage())
			ec.exporter.AvgUsage.WithLabelValues(append([]string{rn, id, node})...).Add(ri.AvgUsage())
			ec.exporter.AvgAvgUsage.WithLabelValues(append([]string{rn, id, node})...).Add(ri.AvgAvgUsage())
		}
		
		ec.exporter.UpdateCount.WithLabelValues(append([]string{name, id, node})...).Add(float64(pod.CI.UpdateCount))	
	}
	return nil
}


func (ec ExporterCollector) Collect(ch chan<- prometheus.Metric) {
	ec.exporter.Limit.Reset()
	ec.exporter.Usage.Reset()
	ec.exporter.AvgUsage.Reset()
	ec.exporter.AvgAvgUsage.Reset()
	ec.exporter.UpdateCount.Reset()
	
	if err := ec.collect(); err != nil {
		klog.Infof("Error reading container stats: %s", err)
	} 
	
	ec.exporter.Limit.Collect(ch)
	ec.exporter.Usage.Collect(ch)
	ec.exporter.AvgUsage.Collect(ch)
	ec.exporter.AvgAvgUsage.Collect(ch)
	ec.exporter.UpdateCount.Collect(ch)	
}


func NewExporter(reg prometheus.Registerer, m *Monitor) *Exporter {
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
	ec := ExporterCollector{exporter: dm, monitor: m}

	reg.MustRegister(ec)
	return dm
}

func ExporterRun(m *Monitor, stopCh <-chan struct{}) {

	klog.Info("Starting Exporter")

	reg := prometheus.NewPedanticRegistry()
	NewExporter(reg, m)
	reg.MustRegister(
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		prometheus.NewGoCollector(),
	)
	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	go http.ListenAndServe(":9091", nil)


	klog.Info("Started Exporter")
	<-stopCh
	klog.Info("Shutting down Exporter")
}