package kuscale

// import (
// 	"fmt"
// 	"log"
// 	"github.com/vishvananda/netlink"
// 	// "net"
//     "runtime"
//     "github.com/vishvananda/netns"
// 	"golang.org/x/sys/unix"
// 	"strings"
// 	"os"
// 	"bufio"
// 	"path"
// 	"strconv"
// )


// func printIpNeigh() {
	
// 	var msg netlink.Ndmsg
// 	neighs, _ := netlink.NeighListExecute(msg)

// 	for nm, neigh := range neighs {
// 		text := neigh.String()

// 		klog.Infof(nm, " : ", text)
// 	}
// }

// func printIpRoute() {
	
// 	routes, _ := netlink.RouteList(nil, 0)

// 	for nm, route := range routes {
// 		text := route.String()

// 		klog.Infof(nm, " : ", text)
// 	}
// }

// func getIfnameByIndex(ifindex int) string {
// 	iface, err:= net.InterfaceByIndex(ifindex)
// 	if err != nil {
// 		return ""
// 	}
// 	// klog.Infof("Found IFname : ", iface.Name)
// 	return iface.Name
// }

// func getIfindexFromRoute(IP string) int {
// 	ip := net.ParseIP(IP)

// 	routes, _ := netlink.RouteList(nil, 0)

// 	for _, route := range routes {
// 		if route.Dst != nil {
// 			if ip.Equal(route.Dst.IP) {
// 				// klog.Infof("Found IF : ", route.LinkIndex)
// 				return route.LinkIndex
// 			}
// 		}
// 	}
// 	return -1
// }

// func printIpLink() {
	
// 	links, _ := netlink.LinkList()

// 	for nm, link := range links {
// 		klog.Infof(nm, " : ", link)
// 	}
// }

// func Setns(ns netns.NsHandle) (err error) {
// 	return unix.Setns(int(ns), 0x40000000)
// }

// func enterNsPrintFromPid(pid int){
// 	runtime.LockOSThread()
//     defer runtime.UnlockOSThread()

// 	// Save the current network namespace
//     origns, _ := netns.Get()
//     defer origns.Close()

// 	ifaces2, _ := net.Interfaces()
//     fmt.Printf("Origin Interfaces: %v\n", ifaces2)

// 	netns, _ := netns.GetFromPath(fmt.Sprintf("/home/proc/%d/ns/net", pid))
// 	Setns(netns)

// 	ifaces, _ := net.Interfaces()
//     fmt.Printf("In Network Namespace Interfaces: %v\n", ifaces)

// 	Setns(origns)
// }

// const latencyInMillis = 25

// func UpdateIngressQdisc(rateInBits, burstInBits uint64, hostDeviceName string) error {
// 	hostDevice, err := netlink.LinkByName(hostDeviceName)
// 	if err != nil {
// 		return fmt.Errorf("get host device: %s", err)
// 	}
// 	return updateTBF(rateInBits, burstInBits, hostDevice.Attrs().Index)
// }

// func updateTBF(rateInBits, burstInBits uint64, linkIndex int) error {
// 	// Equivalent to
// 	// tc qdisc add dev link root tbf
// 	//		rate netConf.BandwidthLimits.Rate
// 	//		burst netConf.BandwidthLimits.Burst
// 	if rateInBits <= 0 {
// 		return fmt.Errorf("invalid rate: %d", rateInBits)
// 	}
// 	if burstInBits <= 0 {
// 		return fmt.Errorf("invalid burst: %d", burstInBits)
// 	}
// 	rateInBytes := rateInBits / 8
// 	burstInBytes := burstInBits / 8
// 	bufferInBytes := buffer(uint64(rateInBytes), uint32(burstInBytes))
// 	latency := latencyInUsec(latencyInMillis)
// 	limitInBytes := limit(uint64(rateInBytes), latency, uint32(burstInBytes))

// 	qdisc := &netlink.Tbf{
// 		QdiscAttrs: netlink.QdiscAttrs{
// 			LinkIndex: linkIndex,
// 			Handle:    netlink.MakeHandle(1, 0),
// 			Parent:    netlink.HANDLE_ROOT,
// 		},
// 		Limit:  uint32(limitInBytes),
// 		Rate:   uint64(rateInBytes),
// 		Buffer: uint32(bufferInBytes),
// 	}
// 	err := netlink.QdiscChange(qdisc)
// 	if err != nil {
// 		return fmt.Errorf("change qdisc: %s", err)
// 	}
// 	return nil
// }

// func SetFirstRx(pi *PodInfo, rxLimit float64) error {
// 	rxBits := uint64(rxLimit) * miliRX
// 	err := CreateIngressQdisc(rxBits, 2 * rxBits, pi.interfaceName)
// 	if err != nil {
// 		return fmt.Errorf("SetFirstRx : %s", err) 
// 	}
// 	pi.CI.RIs["RX"].SetLimit(rxLimit)
// 	return nil
// }

// func CreateIngressQdisc(rateInBits, burstInBits uint64, hostDeviceName string) error {
// 	hostDevice, err := netlink.LinkByName(hostDeviceName)
// 	if err != nil {
// 		return fmt.Errorf("get host device: %s", err)
// 	}
// 	return createTBF(rateInBits, burstInBits, hostDevice.Attrs().Index)
// }

// func createTBF(rateInBits, burstInBits uint64, linkIndex int) error {
// 	// Equivalent to
// 	// tc qdisc add dev link root tbf
// 	//		rate netConf.BandwidthLimits.Rate
// 	//		burst netConf.BandwidthLimits.Burst
// 	if rateInBits <= 0 {
// 		return fmt.Errorf("invalid rate: %d", rateInBits)
// 	}
// 	if burstInBits <= 0 {
// 		return fmt.Errorf("invalid burst: %d", burstInBits)
// 	}
// 	rateInBytes := rateInBits / 8
// 	burstInBytes := burstInBits / 8
// 	bufferInBytes := buffer(uint64(rateInBytes), uint32(burstInBytes))
// 	latency := latencyInUsec(latencyInMillis)
// 	limitInBytes := limit(uint64(rateInBytes), latency, uint32(burstInBytes))

// 	qdisc := &netlink.Tbf{
// 		QdiscAttrs: netlink.QdiscAttrs{
// 			LinkIndex: linkIndex,
// 			Handle:    netlink.MakeHandle(1, 0),
// 			Parent:    netlink.HANDLE_ROOT,
// 		},
// 		Limit:  uint32(limitInBytes),
// 		Rate:   uint64(rateInBytes),
// 		Buffer: uint32(bufferInBytes),
// 	}
// 	err := netlink.QdiscAdd(qdisc)
// 	if err != nil {
// 		return fmt.Errorf("create qdisc: %s", err)
// 	}
// 	return nil
// }

// func tick2Time(tick uint32) uint32 {
// 	return uint32(float64(tick) / float64(netlink.TickInUsec()))
// }

// func time2Tick(time uint32) uint32 {
// 	return uint32(float64(time) * float64(netlink.TickInUsec()))
// }

// func buffer(rate uint64, burst uint32) uint32 {
// 	return time2Tick(uint32(float64(burst) * float64(netlink.TIME_UNITS_PER_SEC) / float64(rate)))
// }

// func limit(rate uint64, latency float64, buffer uint32) uint32 {
// 	return uint32(float64(rate)*latency/float64(netlink.TIME_UNITS_PER_SEC)) + buffer
// }

// func latencyInUsec(latencyInMillis float64) float64 {
// 	return float64(netlink.TIME_UNITS_PER_SEC) * (latencyInMillis / 1000.0)
// }



// type InterfaceStats struct {
// 	// The name of the interface.
// 	Name string `json:"name"`
// 	// Cumulative count of bytes received.
// 	RxBytes uint64 `json:"rx_bytes"`
// 	// Cumulative count of packets received.
// 	RxPackets uint64 `json:"rx_packets"`
// 	// Cumulative count of receive errors encountered.
// 	RxErrors uint64 `json:"rx_errors"`
// 	// Cumulative count of packets dropped while receiving.
// 	RxDropped uint64 `json:"rx_dropped"`
// 	// Cumulative count of bytes transmitted.
// 	TxBytes uint64 `json:"tx_bytes"`
// 	// Cumulative count of packets transmitted.
// 	TxPackets uint64 `json:"tx_packets"`
// 	// Cumulative count of transmit errors encountered.
// 	TxErrors uint64 `json:"tx_errors"`
// 	// Cumulative count of packets dropped while transmitting.
// 	TxDropped uint64 `json:"tx_dropped"`
// }

// var ignoredDevicePrefixes = []string{"lo", "veth", "docker", "tunl0"}

// func isIgnoredDevice(ifName string) bool {
// 	for _, prefix := range ignoredDevicePrefixes {
// 		if strings.HasPrefix(strings.ToLower(ifName), prefix) {
// 			return true
// 		}
// 	}
// 	return false
// }

// func setInterfaceStatValues(fields []string, pointers []*uint64) error {
// 	for i, v := range fields {
// 		val, err := strconv.ParseUint(v, 10, 64)
// 		if err != nil {
// 			return err
// 		}
// 		*pointers[i] = val
// 	}
// 	return nil
// }

// func scanInterfaceStats(netStatsFile string) ([]InterfaceStats, error) {
// 	file, err := os.Open(netStatsFile)
// 	if err != nil {
// 		return nil, fmt.Errorf("failure opening %s: %v", netStatsFile, err)
// 	}
// 	defer file.Close()

// 	scanner := bufio.NewScanner(file)

// 	// Discard header lines
// 	for i := 0; i < 2; i++ {
// 		if b := scanner.Scan(); !b {
// 			return nil, scanner.Err()
// 		}
// 	}
// 	stats := []InterfaceStats{}
// 	for scanner.Scan() {
// 		line := scanner.Text()
// 		line = strings.Replace(line, ":", "", -1)
// 		fields := strings.Fields(line)
	
// 		if len(fields) != 17 {
// 			return nil, fmt.Errorf("invalid interface stats line: %v", line)
// 		}

// 		devName := fields[0]
// 		if isIgnoredDevice(devName) {
// 			continue
// 		}

// 		i := InterfaceStats{
// 			Name: devName,
// 		}

// 		statFields := append(fields[1:5], fields[9:13]...)
// 		statPointers := []*uint64{
// 			&i.RxBytes, &i.RxPackets, &i.RxErrors, &i.RxDropped,
// 			&i.TxBytes, &i.TxPackets, &i.TxErrors, &i.TxDropped,
// 		}

// 		err := setInterfaceStatValues(statFields, statPointers)
// 		if err != nil {
// 			return nil, fmt.Errorf("cannot parse interface stats (%v): %v", err, line)
// 		}

// 		stats = append(stats, i)
// 	}

// 	return stats, nil
// }

// func GetnetworkStats(pi *PodInfo) ([]InterfaceStats, error) {

// 	pid := pi.pid
// 	netStatsFile := path.Join("/home/proc/", pid, "/net/dev")

// 	ifaceStats, err := scanInterfaceStats(netStatsFile)
// 	if err != nil {
// 		fmt.Printf("couldn't read network stats: %v", err)
// 		return nil, err
// 	}
	
// 	return ifaceStats, nil
// }