package vlan

import (
	"github.com/vishvananda/netlink"
	log "github.com/golang/glog"

	"fmt"
	"syscall"
	"net"
	"github.com/coreos/flannel/pkg/ip"
)

type vlanDeviceAttrs struct {
	vlanId uint32
	name   string
}

type vlanDevice struct {
	link  *netlink.Vlan
}
func (dev *vlanDevice) MACAddr() net.HardwareAddr {
	return dev.link.HardwareAddr
}
func newVLANDevice(devAttrs *vlanDeviceAttrs) (*vlanDevice, error) {
	link := &netlink.Vlan{
		LinkAttrs: netlink.LinkAttrs{
			Name: devAttrs.name,
		},
		VlanId:      int(devAttrs.vlanId),
	}

	link, err := ensureLink(link)
	if err != nil {
		return nil, err
	}
	//// this enables ARP requests being sent to userspace via netlink
	//sysctlPath := fmt.Sprintf("/proc/sys/net/ipv4/neigh/%s/app_solicit", devAttrs.name)
	//if err := sysctlSet(sysctlPath, "3"); err != nil {
	//	return nil, err
	//}

	return &vlanDevice{
		link: link,
	}, nil
}

func ensureLink(vlan *netlink.Vlan) (*netlink.Vlan, error) {
	err := netlink.LinkAdd(vlan)
	if err == syscall.EEXIST {
		// it's ok if the device already exists as long as config is similar
		existing, err := netlink.LinkByName(vlan.Name)
		if err != nil {
			return nil, err
		}

		incompat := vlanLinksIncompat(vlan, existing)
		if incompat == "" {
			return existing.(*netlink.Vlan), nil
		}

		// delete existing
		log.Warningf("%q already exists with incompatable configuration: %v; recreating device", vlan.Name, incompat)
		if err = netlink.LinkDel(existing); err != nil {
			return nil, fmt.Errorf("failed to delete interface: %v", err)
		}

		// create new
		if err = netlink.LinkAdd(vlan); err != nil {
			return nil, fmt.Errorf("failed to create vlan interface: %v", err)
		}
	} else if err != nil {
		return nil, err
	}

	ifindex := vlan.Index
	link, err := netlink.LinkByIndex(vlan.Index)
	if err != nil {
		return nil, fmt.Errorf("can't locate created vlan device with index %v", ifindex)
	}

	var ok bool
	if vlan, ok = link.(*netlink.Vlan); !ok {
		return nil, fmt.Errorf("created vlan device with index %v is not vlan", ifindex)
	}

	return vlan, nil
}

func vlanLinksIncompat(l1, l2 netlink.Link) string {
	if l1.Type() != l2.Type() {
		return fmt.Sprintf("link type: %v vs %v", l1.Type(), l2.Type())
	}

	v1 := l1.(*netlink.Vlan)
	v2 := l2.(*netlink.Vlan)

	if v1.VlanId != v2.VlanId {
		return fmt.Sprintf("VlanId: %v vs %v", v1.VlanId, v2.VlanId)
	}

	if v1.Name != v2.Name {
		return fmt.Sprintf("vlan name: %s vs %s", v1.Name, v2.Name)
	}

	return ""
}

func (dev *vlanDevice) Configure(ipn ip.IP4Net) error {
	setAddr4(dev.link, ipn.ToIPNet())
	setGateway(dev.link,nil)

	if err := netlink.LinkSetUp(dev.link); err != nil {
		return fmt.Errorf("failed to set interface %s to UP state: %s", dev.link.Attrs().Name, err)
	}

	// explicitly add a route since there might be a route for a subnet already
	// installed by Docker and then it won't get auto added
	route := netlink.Route{
		LinkIndex: dev.link.Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       ipn.Network().ToIPNet(),
	}
	if err := netlink.RouteAdd(&route); err != nil && err != syscall.EEXIST {
		return fmt.Errorf("failed to add route (%s -> %s): %v", ipn.Network().String(), dev.link.Attrs().Name, err)
	}

	return nil
}

// sets IP4 addr on link removing any existing ones first
func setAddr4(link *netlink.Vlan, ipn *net.IPNet) error {
	addrs, err := netlink.AddrList(link, syscall.AF_INET)
	if err != nil {
		return err
	}

	for _, addr := range addrs {
		if err = netlink.AddrDel(link, &addr); err != nil {
			return fmt.Errorf("failed to delete IPv4 addr %s from %s", addr.String(), link.Attrs().Name)
		}
	}
	// Ensure that the device has a /32 address so that no broadcast routes are created.
	// This IP is just used as a source address for host to workload traffic (so
	// the return path for the traffic has a decent address to use as the destination)
	ipn.Mask = net.CIDRMask(32, 32)
	addr := netlink.Addr{IPNet: ipn, Label: ""}
	if err = netlink.AddrAdd(link, &addr); err != nil {
		return fmt.Errorf("failed to add IP address %s to %s: %s", ipn.String(), link.Attrs().Name, err)
	}

	return nil
}

func setGateway(link *netlink.Vlan , gateway *net.IPNet) error{
	return nil
}