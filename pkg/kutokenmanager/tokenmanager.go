package KuTokenManager

import (
	"fmt"
	"log"
	"net"
	"os"
	"path"

	// "path/filepath"
	// "strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/klog"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1alpha"
)

type KuTokenManager struct {
	socketFile            string
	resourceName          string
	resourceSize          int
	allocatedPods         int
	totalAllocatedDevices int
	server                *grpc.Server
	stop                  chan interface{}
}

func NewKuTokenManager(resourceName string, resourceSize int, socketFile string) *KuTokenManager {
	return &KuTokenManager{
		resourceName: resourceName,
		resourceSize: resourceSize,
		socketFile:   socketFile,
		server:       nil,
		stop:         nil,
	}
}

func (ktm *KuTokenManager) initialize() {
	ktm.server = grpc.NewServer([]grpc.ServerOption{}...)
	ktm.stop = make(chan interface{})
	ktm.update = make(chan int)
}

func (ktm *KuTokenManager) cleanup() {
	close(ktm.stop)
	close(ktm.update)
	ktm.server = nil
	ktm.stop = nil
}

// ListAndWatch lists devices and update that list according to the health status

func (ktm *KuTokenManager) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	// log.Printf("ListAndWatch Function create %d %s resource", ktm.resourceSize, ktm.resourceName)
	defaultDevices := make([]*pluginapi.Device, ktm.resourceSize)
	for i := 0; i < ktm.resourceSize; i++ {
		defaultDevices[i] = &pluginapi.Device{
			ID:     fmt.Sprintf("%s-%d", ktm.resourceName, i),
			Health: pluginapi.Healthy,
		}
	}

	s.Send(&pluginapi.ListAndWatchResponse{Devices: defaultDevices})

	for {
		select {
		case <-ktm.stop:
			return nil
		}
	}
	return nil
}

// Allocate which return list of devices.
func (ktm *KuTokenManager) Allocate(ctx context.Context, reqs *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {

	/* Enable GPU Module */
	createGpuMod(ktm.totalAllocatedDevices)

	responses := pluginapi.AllocateResponse{}

	for _, req := range reqs.ContainerRequests {
		log.Printf("Allocate %d %s resource", len(req.DevicesIDs), ktm.resourceName)
		responses.ContainerResponses = append(responses.ContainerResponses, &pluginapi.ContainerAllocateResponse{
			Mounts: []*pluginapi.Mount{
				{
					ContainerPath: "/home/gpu",
					HostPath:      fmt.Sprintf("/sys/kernel/gpu/containers/%d", ktm.totalAllocatedDevices),
				},
			},
		})
	}

	ktm.allocatedPods = ktm.allocatedPods + 1
	ktm.totalAllocatedDevices = ktm.totalAllocatedDevices + 1

	return &responses, nil
}

// GetDevicePluginOptions is unimplemented for this plugin
func (ktm *KuTokenManager) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

// PreStartContainer is unimplemented for this plugin
func (ktm *KuTokenManager) PreStartContainer(ctx context.Context, r *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

// GetPreferredAllocation is unimplemented for this plugin
func (ktm *KuTokenManager) GetPreferredAllocation(ctx context.Context, r *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	return &pluginapi.PreferredAllocationResponse{}, nil
}

// dial establishes the gRPC communication with the registered device plugin.
func (ktm *KuTokenManager) dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	c, err := grpc.Dial(unixSocketPath, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(timeout),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	if err != nil {
		return nil, err
	}

	return c, nil
}

// Serve starts the gRPC server of the device plugin.
func (ktm *KuTokenManager) Serve() error {
	os.Remove(ktm.socketFile)
	sock, err := net.Listen("unix", ktm.socketFile)
	if err != nil {
		return err
	}

	pluginapi.RegisterDevicePluginServer(ktm.server, ktm)

	go func() {
		lastCrashTime := time.Now()
		restartCount := 0
		for {
			// log.Printf("Starting GRPC server for '%s'", ktm.resourceName)
			err := ktm.server.Serve(sock)
			if err == nil {
				break
			}

			// restart if it has not been too often
			// i.e. if server has crashed more than 5 times and it didn't last more than one hour each time
			if restartCount > 5 {
				// quit
				klog.Fatalf("GRPC server for '%s' has repeatedly crashed recently. Quitting", ktm.resourceName)
			}
			timeSinceLastCrash := time.Since(lastCrashTime).Seconds()
			lastCrashTime = time.Now()
			if timeSinceLastCrash > 3600 {
				// it has been one hour since the last crash.. reset the count
				// to reflect on the frequency
				restartCount = 1
			} else {
				restartCount++
			}
		}
	}()

	// Wait for server to start by launching a blocking connexion
	conn, err := ktm.dial(ktm.socketFile, 5*time.Second)
	if err != nil {
		return err
	}
	conn.Close()

	return nil
}

// Register registers the device plugin for the given resourceName with Kubelet.
func (ktm *KuTokenManager) Register() error {
	conn, err := ktm.dial(pluginapi.KubeletSocket, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(ktm.socketFile),
		ResourceName: ktm.resourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return err
	}
	return nil
}

func (ktm *KuTokenManager) Start() error {
	ktm.initialize()

	// insmod GPU Abstraction Layer

	err := ktm.Serve()
	if err != nil {
		klog.Infof("Could not start device plugin for '%s': %s", ktm.resourceName, err)
		ktm.cleanup()
		return err
	}
	// log.Printf("Starting to serve '%s' on %s", ktm.resourceName, ktm.socketFile)

	err = ktm.Register()
	if err != nil {
		klog.Infof("Could not register device plugin: %s", err)
		ktm.Stop()
		return err
	}
	// log.Printf("Registered device plugin for '%s' with Kubelet", ktm.resourceName)

	// go ktm.CheckHealth(ktm.stop, ktm.cachedDevices, ktm.health)

	return nil
}

// Stop stops the gRPC server.
func (ktm *KuTokenManager) Stop() error {
	if ktm == nil || ktm.server == nil {
		return nil
	}
	klog.Infof("Stopping KuTokenManager to serve '%s' on %s", ktm.resourceName, ktm.socketFile)
	ktm.server.Stop()
	if err := os.Remove(ktm.socketFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	ktm.cleanup()
	return nil
}

func (ktm *KuTokenManager) Run(stopCh <-chan struct{}) {

	insmodGpuMod()

	if err := ktm.Start(); err != nil {
		klog.Infof("Could not contact Kubelet")
	}

	klog.V(4).Info("Started KuTokenManager")
	<-stopCh
	ktm.Stop()
	rmmodGpuMod()
	klog.V(4).Info("Shutting KuTokenManager down")
}
