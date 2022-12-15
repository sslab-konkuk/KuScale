package kuprofiler

import (
	"fmt"
	"os"
	"time"

	hist "github.com/aybabtme/uniplot/histogram"
	"k8s.io/klog"
)

var li *LatencyInfo

type latencyData_st struct {
	name      string
	startTime int64
	duration  []float64
}

type LatencyInfo struct {
	enableFlag  bool
	initTime    int64
	latencyData map[string]latencyData_st
}

func NewLatencyInfo(flag bool) {
	if flag {
		klog.V(4).Info("Start to Trace Latency")
	} else {
		klog.V(4)
	}
	li = &LatencyInfo{enableFlag: flag,
		latencyData: make(map[string]latencyData_st),
		initTime:    time.Now().UnixNano(),
	}
}

func StartTime() int64 {
	if !li.enableFlag {
		return 0
	} else {
		return time.Now().UnixNano()
	}
}

func Record(funcName string, startTime int64) {
	if !li.enableFlag {
		return
	}
	current := time.Now().UnixNano()

	latencyData, ok := li.latencyData[funcName]
	if !ok {
		latencyData = latencyData_st{name: funcName, duration: make([]float64, 0)}
	}

	elapsedTime := float64(current - startTime)
	klog.V(5).Info("Record ", funcName, " : ", startTime-li.initTime, ": ", elapsedTime)
	latencyData.duration = append(latencyData.duration, elapsedTime)
	li.latencyData[funcName] = latencyData
	return
}

func RecordStart(funcName string) {
	if !li.enableFlag {
		return
	}
	current := time.Now().UnixNano()

	latencyData, ok := li.latencyData[funcName]
	if !ok {
		latencyData = latencyData_st{name: funcName, duration: make([]float64, 0)}
	}
	latencyData.startTime = current
	klog.V(5).Info("RecordStart", funcName, " : ", latencyData.startTime)
	li.latencyData[funcName] = latencyData
	return
}

func RecordEnd(funcName string) {
	if !li.enableFlag {
		return
	}
	current := time.Now().UnixNano()

	latencyData, ok := li.latencyData[funcName]
	if !ok {
		latencyData = latencyData_st{name: funcName, duration: make([]float64, 0)}
	}
	if latencyData.startTime == 0 {
		return
	}
	elapsedTime := float64(current - latencyData.startTime)
	klog.V(5).Info("RecordEnd ", funcName, ": ", elapsedTime)
	latencyData.duration = append(latencyData.duration, elapsedTime)
	latencyData.startTime = 0
	li.latencyData[funcName] = latencyData
	return
}

func Summary() {
	if !li.enableFlag {
		return
	}
	for name, data := range li.latencyData {
		fmt.Fprintf(os.Stdout, "\n Hitogram Func [%s] \n", name)
		histdata := hist.Hist(10, data.duration)
		_ = hist.Fprintf(os.Stdout, histdata, hist.Linear(50), func(v float64) string {
			return time.Duration(v).String()
		})

		sum := 0.
		for _, duration := range data.duration {
			sum += duration
		}
		fmt.Fprintln(os.Stdout, name, "Average :", time.Duration(sum/float64(len(data.duration))).String())
		// fmt.Fprintln(os.Stdout, data)
	}
}
