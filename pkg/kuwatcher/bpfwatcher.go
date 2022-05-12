package kuwatcher

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"k8s.io/klog"

	bpf "github.com/iovisor/gobpf/bcc"
)

const source string = `
	#include <uapi/linux/ptrace.h>
	struct cuLaunchKernel_event_t {
			u32 pid;
			u64 ts;
	} __attribute__((packed));
	BPF_HASH(pidmap, u32);
	BPF_PERF_OUTPUT(cuLaunchKernel_events);
	int trace_cuLaunchKernel(void *ctx) {
			struct cuLaunchKernel_event_t event = {};
			u32 pid;
			pid = bpf_get_current_pid_tgid();
			u64 *deep;
			deep = pidmap.lookup(&pid);
			if(deep == NULL){
				u64 delta = 1;
				pidmap.update(&pid, &delta);
				event.pid = pid;
				event.ts = bpf_ktime_get_ns() / 1000;
				cuLaunchKernel_events.perf_submit(ctx, &event, sizeof(event));
			}
			return 0;
	}
`

type cuLaunchKernelEvent struct {
	Pid uint32
	Ts  uint64
}

type BpfWatcher struct {
	tracePID []uint32
}

func NewbpfWatcher() *BpfWatcher {
	bpfWatcher := BpfWatcher{}

	return &bpfWatcher
}

func (bw *BpfWatcher) Run(stopCh <-chan struct{}) {
	m := bpf.NewModule(source, []string{})
	defer m.Close()

	Uprobe, err := m.LoadUprobe("trace_cuLaunchKernel")
	if err != nil {
		klog.V(4).Infof("Failed to load cuLaunchKernel: %s\n", err)
		os.Exit(1)
	}

	err = m.AttachUprobe("/kubeshare/library/libgemhook.so.1", "cuLaunchKernel", Uprobe, -1)
	//_Z22cuLaunchKernel_prehookP9CUfunc_stjjjjjjjP11CUstream_stPPvS4_
	if err != nil {
		klog.V(4).Infof("Failed to attach return_value: %s\n", err)
		os.Exit(1)
	}

	table := bpf.NewTable(m.TableId("cuLaunchKernel_events"), m)

	channel := make(chan []byte)

	perfMap, err := bpf.InitPerfMap(table, channel, nil)
	if err != nil {
		klog.V(4).Infof("Failed to init perf map: %s\n", err)
		os.Exit(1)
	}

	// klog.V(4).Infof("%10s\t%s\n", "PID", "TS")
	go func() {
		var event cuLaunchKernelEvent
		for {
			data := <-channel
			err := binary.Read(bytes.NewBuffer(data), binary.LittleEndian, &event)
			if err != nil {
				klog.V(4).Infof("failed to decode received data: %s\n", err)
				continue
			}
			klog.V(4).Infof("Detected %d's First cuLaunchKernel [ID : %s]", event.Pid, bw.getIDfromPID(event.Pid))
		}
	}()

	perfMap.Start()
	klog.V(4).Info("Starting bpfWatcher")
	<-stopCh
	perfMap.Stop()
	klog.V(4).Info("Shutting bpfWatcher Down")
}

func (bw *BpfWatcher) getIDfromPID(pid uint32) string {
	data, _ := ioutil.ReadFile(fmt.Sprintf("/home/proc/%d/cgroup", pid))
	lines := strings.Split(string(data), "\n")
	for i := range lines {
		if strings.Contains(lines[i], "cpu") {
			docker := strings.Split(lines[i], "-")
			pidString := strings.Split(docker[1], ".")
			klog.V(5).Info(pidString[0])
			return pidString[0]
		}
	}
	return ""
}
