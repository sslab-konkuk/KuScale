package kuwatcher

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc"
	"k8s.io/klog"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
)

const (
	socketPath        = "/var/lib/kubelet/pod-resources/kubelet.sock"
	tokenName         = "kuscale.com/token"
	connectionTimeout = 10 * time.Second
)

type podWathcer struct {
	updatedPodMap map[string]string

	client podresourcesapi.PodResourcesListerClient
	conn   *grpc.ClientConn
	ctx    context.Context
}

var pw *podWathcer

/*
Func Name : PodWatcher()
Objective : 1) Initialize Pod Watcher
*/

func PodWatcher(stopCh, tokenReqCh, newPodCh chan string) {
	pw = &podWathcer{updatedPodMap: make(map[string]string)}
	pw.ctx = context.Background()

	var err error
	pw.conn, err = grpc.DialContext(pw.ctx, socketPath, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	if err != nil {
		klog.Errorf("failure connecting to %s: %v", socketPath, err)
	}

	pw.client = podresourcesapi.NewPodResourcesListerClient(pw.conn)

	pw.updatedPodMap = make(map[string]string)

	klog.V(4).Info("Starting PodWatcher")
	for {
		select {
		case <-stopCh:
			pw.conn.Close()
			klog.V(4).Info("Shutting PodWatcher Down")
			return
		case <-tokenReqCh:
			klog.V(4).Info("Get New Token Request : ", tokenReqCh)
			pw.GetNewPod(newPodCh)
		}
	}
}

func (pw *podWathcer) GetNewPod(newPodCh chan string) ([]string, error) {

	resp, err := pw.client.List(pw.ctx, &podresourcesapi.ListPodResourcesRequest{})
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
				newPodCh <- podName
			}
		}
	}
	return ret, nil
}
