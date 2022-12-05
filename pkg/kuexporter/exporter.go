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
package kumexporter

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	kumonitor "github.com/sslab-konkuk/KuScale/pkg/kumonitor"
	"k8s.io/klog"
)

type Exporter struct {
	Limit            *prometheus.CounterVec
	Usage            *prometheus.CounterVec
	AvgUsage         *prometheus.CounterVec
	DynamicWeight    *prometheus.CounterVec
	UpdatedCount     *prometheus.GaugeVec
	TokenReservation *prometheus.GaugeVec
	TokenQueue       *prometheus.GaugeVec
}

type ExporterCollector struct {
	exporter         *Exporter
	connectedMonitor *kumonitor.Monitor
	nodeName         string
}

func (ec ExporterCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(ec, ch)
}

func (ec ExporterCollector) collect() error {
	for _, pod := range ec.connectedMonitor.RunningPodMap {
		name := pod.PodName
		id := pod.PodName
		node := ec.nodeName

		for rn, ri := range pod.RIs {
			resourceName := string(rn)
			ec.exporter.Limit.WithLabelValues([]string{resourceName, id, node}...).Add(ri.Limit())
			ec.exporter.Usage.WithLabelValues([]string{resourceName, id, node}...).Add(ri.Usage())
			ec.exporter.AvgUsage.WithLabelValues([]string{resourceName, id, node}...).Add(ri.AvgUsage())
			ec.exporter.DynamicWeight.WithLabelValues([]string{resourceName, id, node}...).Add(ri.DynamicWeight())
		}

		ec.exporter.UpdatedCount.WithLabelValues([]string{name, id, node}...).Add(float64(pod.UpdatedCount))
		ec.exporter.TokenReservation.WithLabelValues([]string{name, id, node}...).Add(pod.TokenReservation)
		ec.exporter.TokenQueue.WithLabelValues([]string{name, id, node}...).Add(pod.TokenQueue)
	}
	return nil
}

func (ec ExporterCollector) Collect(ch chan<- prometheus.Metric) {
	ec.exporter.Limit.Reset()
	ec.exporter.Usage.Reset()
	ec.exporter.AvgUsage.Reset()
	ec.exporter.DynamicWeight.Reset()
	ec.exporter.UpdatedCount.Reset()
	ec.exporter.TokenReservation.Reset()
	ec.exporter.TokenQueue.Reset()

	if err := ec.collect(); err != nil {
		klog.Infof("Error reading container stats: %s", err)
	}

	ec.exporter.Limit.Collect(ch)
	ec.exporter.Usage.Collect(ch)
	ec.exporter.AvgUsage.Collect(ch)
	ec.exporter.DynamicWeight.Collect(ch)
	ec.exporter.UpdatedCount.Collect(ch)
	ec.exporter.TokenReservation.Collect(ch)
	ec.exporter.TokenQueue.Collect(ch)
}

func NewExporter(reg prometheus.Registerer, m *kumonitor.Monitor, nodeName string) *Exporter {
	dm := &Exporter{
		Limit: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "Limit",
			Help: "Resource Limit",
		}, []string{"name", "id", "node"},
		),
		Usage: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "Usage",
			Help: "Resource Usage",
		},
			[]string{"name", "id", "node"},
		),
		AvgUsage: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "AvgUsage",
			Help: "Resource AvgUsage",
		},
			[]string{"name", "id", "node"},
		),
		DynamicWeight: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "DynamicWeight",
			Help: "Resource DynamicWeight",
		},
			[]string{"name", "id", "node"},
		),
		UpdatedCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "UpdatedCount",
			Help: "UpdatedCount",
		},
			[]string{"name", "id", "node"},
		),
		TokenReservation: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "TokenReservation",
			Help: "TokenReservation",
		},
			[]string{"name", "id", "node"},
		),
		TokenQueue: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "TokenQueue",
			Help: "TokenQueue",
		},
			[]string{"name", "id", "node"},
		),
	}
	ec := ExporterCollector{exporter: dm, connectedMonitor: m, nodeName: nodeName}

	reg.MustRegister(ec)
	return dm
}

func ExporterRun(m *kumonitor.Monitor, nodeName string, stopCh <-chan struct{}) {

	klog.V(4).Info("Starting Exporter")

	reg := prometheus.NewPedanticRegistry()
	NewExporter(reg, m, nodeName)
	reg.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
	)
	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	go http.ListenAndServe(":9091", nil)

	klog.V(4).Info("Started Exporter")
	<-stopCh
	klog.V(4).Info("Shutting exporter down")
}
