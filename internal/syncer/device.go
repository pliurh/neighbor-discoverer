package syncer

import (
	log "k8s.io/klog/v2"

	"github.com/vishvananda/netlink"

	neighbordiscovererv1 "github.com/pliurh/neighbor-discoverer/api/v1"
)

const (
	sysBusPciDevices = "/sys/bus/pci/devices"
	sysClassNet      = "/sys/class/net"
	netClass         = 0x02
)

// DiscoverNetworkDevices discovers local device links
func DiscoverNetworkDevices(status *neighbordiscovererv1.NodeNeighborNetworkStatus) ([]netlink.Link, error) {
	devLinks := []netlink.Link{}
	links, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		if link.Type() != "device" || link.Attrs().Name == "lo" {
			// It is not a pci device, ignore
			continue
		}
		log.V(0).Infof("Discover interface %s", link.Attrs().Name)
		status.Interfaces = append(status.Interfaces,
			neighbordiscovererv1.InterfaceStatus{
				Name:         link.Attrs().Name,
				HardwareAddr: link.Attrs().HardwareAddr.String(),
			})
		devLinks = append(devLinks, link)
	}
	return devLinks, nil
}
