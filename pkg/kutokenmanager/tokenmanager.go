package kutokenmanager

import (
	"fmt"
	"net"
	"os"
	"path"

	// "path/filepath"
	// "strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/klog"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type KuTokenManager struct {
	socketFile                 string
	tokenName                  string
	tokenSize                  int
	totalIDs                   int
	server                     *grpc.Server
	stop                       chan interface{}
	newPodCh                   chan string
	health                     chan string
	healthCheckIntervalSeconds time.Duration
}

func NewKuTokenManager(tokenName string, tokenSize int, socketFile string) *KuTokenManager {
	return &KuTokenManager{
		tokenName:  tokenName,
		tokenSize:  tokenSize,
		socketFile: socketFile,
		server:     nil,
		stop:       nil,
	}
}

func (ktm *KuTokenManager) cleanup() error {

	if err := os.Remove(ktm.socketFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// Stop stops the gRPC server.
func (ktm *KuTokenManager) Stop() error {
	if ktm == nil || ktm.server == nil {
		return nil
	}
	klog.V(5).Infof("Stopping KuTokenManager to serve '%s' on %s", ktm.tokenName, ktm.socketFile)
	ktm.server.Stop()
	if err := os.Remove(ktm.socketFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	close(ktm.stop)
	ktm.server = nil
	ktm.stop = nil
	return nil
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

// Register registers the device plugin for the given tokenName with Kubelet.
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
		ResourceName: ktm.tokenName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return err
	}
	return nil
}

// Serve starts the gRPC server of the device plugin.
func (ktm *KuTokenManager) Start() error {

	err := ktm.cleanup()
	if err != nil {
		return err
	}

	sock, err := net.Listen("unix", ktm.socketFile)
	if err != nil {
		return err
	}

	ktm.server = grpc.NewServer([]grpc.ServerOption{}...)
	pluginapi.RegisterDevicePluginServer(ktm.server, ktm)
	go ktm.server.Serve(sock)

	// Wait for server to start by launching a blocking connexion
	conn, err := ktm.dial(ktm.socketFile, 5*time.Second)
	if err != nil {
		return err
	}
	conn.Close()

	return nil
}

func (ktm *KuTokenManager) Run(stopCh, newPodCh chan string) {

	InsmodGpuMod()

	ktm.stop = make(chan interface{})
	ktm.newPodCh = newPodCh

	err := ktm.Start()
	if err != nil {
		klog.Infof("Could not start device plugin for '%s': %s", ktm.tokenName, err)
		ktm.cleanup()
		goto ErrorStop
	}

	err = ktm.Register()
	if err != nil {
		klog.Infof("Could not register device plugin: %s", err)
		ktm.Stop()
		goto ErrorStop
	}

	klog.V(5).Info("Started KuTokenManager")
	<-stopCh
	ktm.Stop()
ErrorStop:
	// RmmodGpuMod()
	klog.V(5).Info("Shutted KuTokenManager Down")
}

// func getHostDevicesHealth() string {
// 	health := pluginapi.Healthy
// 	for _, device := range hostDevices {
// 		if _, err := os.Stat(device.HostPath); os.IsNotExist(err) {
// 			health = pluginapi.Unhealthy
// 			klog.V(5).Infof("HostPath not found: %s", device.HostPath)
// 		}
// 	}
// 	return health
// }

// func (ktm *KuTokenManager) healthCheck() {
// 	klog.V(5).Infof("Starting health check every %d seconds", m.healthCheckIntervalSeconds)
// 	ticker := time.NewTicker(ktm.healthCheckIntervalSeconds * time.Second)
// 	lastHealth := ""
// 	for {
// 		select {
// 		case <-ticker.C:
// 			health := getHostDevicesHealth(m.hostDevices)
// 			if lastHealth != health {
// 				klog.V(5).Infof("Health is changed: %s -> %s", lastHealth, health)
// 				ktm.health <- health
// 			}
// 			lastHealth = health
// 		case <-ktm.stop:
// 			ticker.Stop()
// 			return
// 		}
// 	}
// }

// ListAndWatch lists devices and update that list according to the health status
func (ktm *KuTokenManager) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	// log.Printf("ListAndWatch Function create %d %s resource", ktm.tokenSize, ktm.tokenName)
	defaultDevices := make([]*pluginapi.Device, ktm.tokenSize)
	for i := 0; i < ktm.tokenSize; i++ {
		defaultDevices[i] = &pluginapi.Device{
			ID:     fmt.Sprintf("%s-%d", ktm.tokenName, i),
			Health: pluginapi.Healthy,
		}
	}

	s.Send(&pluginapi.ListAndWatchResponse{Devices: defaultDevices})

	ktm.totalIDs = int(GetFileParamUint("/sys/kernel/gpu/configs", "/totalIDs"))
	klog.V(5).Info("totalIDs : ", ktm.totalIDs)

	for {
		select {
		case <-ktm.stop:
			return nil
			// case health := <-ktm.health:
			// 	// Update health of devices only in this thread.
			// 	// for _, dev := range m.devs {
			// 	// 	dev.Health = health
			// 	// }
			// 	// s.Send(&pluginapi.ListAndWatchResponse{Devices: m.devs})
			// 	klog.V(3).Info("Problem with helath in ListAndWatch ", health)
		}
	}
}

// Allocate which return list of devices.
func (ktm *KuTokenManager) Allocate(ctx context.Context, reqs *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {

	/* Enable GPU Module */
	CreateGPUID(fmt.Sprintf("%d", ktm.totalIDs))

	var tokenRes int
	responses := pluginapi.AllocateResponse{}
	vgpuId := ktm.totalIDs

	for _, req := range reqs.ContainerRequests {
		tokenRes = len(req.DevicesIDs)
		klog.V(4).Infof("Allocate %d %s resource to ID : %d", tokenRes, ktm.tokenName, vgpuId)
		responses.ContainerResponses = append(responses.ContainerResponses,
			&pluginapi.ContainerAllocateResponse{
				Envs: map[string]string{
					"LD_PRELOAD":        "/kubeshare/library/libgemhook.so.1",
					"LD_LIBRARY_PATH":   "/kubeshare/library/:$LD_LIBRARY_PATH",
					"GEMINI_IPC_DIR":    "/kubeshare/scheduler/ipc/",
					"GEMINI_GROUP_NAME": fmt.Sprintf("%d", vgpuId),
				},
				Mounts: []*pluginapi.Mount{
					{
						ContainerPath: "/kubeshare", //TODO: Need to change it the specific path
						HostPath:      "/kubeshare",
					},
					{
						ContainerPath: "/ku-gpu", //TODO: Need to change it the specific path
						HostPath:      fmt.Sprintf("/sys/kernel/gpu/IDs/%d", vgpuId),
					},
				},
				Annotations: map[string]string{
					"kuauto.token": fmt.Sprintf("%d", tokenRes),
					"kuauto.vgpu":  fmt.Sprintf("%d", vgpuId),
				},
			},
		)
	}
	ktm.newPodCh <- fmt.Sprintf("%d:%d", vgpuId, tokenRes)
	ktm.totalIDs = ktm.totalIDs + 1
	return &responses, nil
}
