package vlan

import (
	"github.com/coreos/flannel/subnet"
	"golang.org/x/net/context"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/backend"
)

type  network struct {
	backend.SimpleNetwork
	name      string
	extIface  *backend.ExternalInterface
	dev       *vlanDevice
//	routes    routes
	subnetMgr subnet.Manager
}


func (n *network) Lease() *subnet.Lease{
	return nil
}


func (n *network) MTU() int{
	return 0
}
func (n *network) Run(ctx context.Context){

}


type vlanLeaseAttrs struct {
	VtepMAC hardwareAddr
}


func newNetwork(name string, subnetMgr subnet.Manager, extIface *backend.ExternalInterface, dev *vlanDevice, _ ip.IP4Net, lease *subnet.Lease) (*network, error) {
	nw := &network{
		SimpleNetwork: backend.SimpleNetwork{
			SubnetLease: lease,
			ExtIface:    extIface,
		},
		name:      name,
		subnetMgr: subnetMgr,
		dev:       dev,
	}

	return nw, nil
}