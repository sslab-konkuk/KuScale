package kuwatcher

import (
	"context"
	"fmt"
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

var flag int
var updatedPodMap map[string]string

type PodWathcer struct {
	client podresourcesapi.PodResourcesListerClient
}

func connectToServer(socket string) (*grpc.ClientConn, func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, socket, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	if err != nil {
		return nil, func() {}, fmt.Errorf("failure connecting to %s: %v", socket, err)
	}

	return conn, func() { conn.Close() }, nil
}

func getPodInfoFromKubelet() (*podresourcesapi.ListPodResourcesResponse, error) {
	conn, cleanup, err := connectToServer(socketPath)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	client := podresourcesapi.NewPodResourcesListerClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	resp, err := client.List(ctx, &podresourcesapi.ListPodResourcesRequest{})
	if err != nil {
		return nil, fmt.Errorf("failure getting pod resources %v", err)
	}

	return resp, nil
}

func Scan() ([]string, error) {

	var ret []string

	if flag == 0 {
		updatedPodMap = make(map[string]string)
		flag = 1
	}

	resp, err := getPodInfoFromKubelet()
	if err != nil {
		klog.Error("Error in getPodInfoFromKubelet ", err)
		return ret, nil
	}

	for _, pod := range resp.GetPodResources() {
		tokenSize := 0
		podName := pod.GetName()
		for _, container := range pod.GetContainers() {
			for _, device := range container.GetDevices() {
				if device.GetResourceName() == tokenName {
					tokenSize = tokenSize + 1
				}
			}
			if _, ok := updatedPodMap[podName]; ok {
				continue
			}
			if tokenSize > 0 {
				klog.V(4).Infof("Pod: %s, Container: %s , %s:= %d", podName, container.GetName(), tokenName, tokenSize)
				updatedPodMap[podName] = podName
				ret = append(ret, podName)
			}
		}
	}
	return ret, nil
}
