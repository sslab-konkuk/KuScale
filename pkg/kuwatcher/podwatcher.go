package kuwatcher

import (
	"context"
	"net"
	"time"

	"github.com/sslab-konkuk/KuScale/pkg/kuprofiler"
	"google.golang.org/grpc"
	"k8s.io/klog"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
)

const (
	socketPath        = "/var/lib/kubelet/pod-resources/kubelet.sock"
	tokenName         = "kuscale.com/token"
	connectionTimeout = 10 * time.Second
)

type PodWathcer struct {
	updatedPodMap map[string]string

	client podresourcesapi.PodResourcesListerClient
	conn   *grpc.ClientConn
}

var pw *PodWathcer

/*
Func Name : InitPodWatcher()
Objective : 1) Initialize Pod Watcher
*/

func InitPodWatcher() {
	pw = &PodWathcer{updatedPodMap: make(map[string]string)}
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	var err error
	pw.conn, err = grpc.DialContext(ctx, socketPath, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	if err != nil {
		klog.Errorf("failure connecting to %s: %v", socketPath, err)
	}

	pw.client = podresourcesapi.NewPodResourcesListerClient(pw.conn)

	pw.updatedPodMap = make(map[string]string)
}

func Scan() ([]string, error) {
	startTime := kuprofiler.StartTime()
	defer kuprofiler.Record("kuwatcher_scan", startTime)

	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	resp, err := pw.client.List(ctx, &podresourcesapi.ListPodResourcesRequest{})
	if err != nil {
		klog.Errorf("pw.client.List")
	}

	var ret []string

	for _, pod := range resp.GetPodResources() {
		tokenSize := 0
		podName := pod.GetName()
		for _, container := range pod.GetContainers() {
			for _, device := range container.GetDevices() {
				if device.GetResourceName() == tokenName {
					tokenSize = tokenSize + 1
				}
			}
			if _, ok := pw.updatedPodMap[podName]; ok {
				continue
			}
			if tokenSize > 0 {
				klog.V(4).Infof("Pod: %s, Container: %s , %s:= %d", podName, container.GetName(), tokenName, tokenSize)
				pw.updatedPodMap[podName] = podName
				ret = append(ret, podName)
			}
		}
	}
	return ret, nil
}

func ExitPodWatcher() {
	pw.conn.Close()
}
