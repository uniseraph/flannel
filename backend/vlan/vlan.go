package vlan

import (
	"github.com/coreos/flannel/subnet"
	"github.com/coreos/flannel/backend"
	"golang.org/x/net/context"
	"fmt"
	"encoding/json"
	"github.com/coreos/flannel/pkg/ip"
	"net"
	"errors"
)
type hardwareAddr net.HardwareAddr

type VlanBackend struct {
	extInterface *backend.ExternalInterface
	subnetMgr  subnet.Manager
}

func New(sm subnet.Manager, extIface *backend.ExternalInterface) (backend.Backend, error) {
	return &VlanBackend{
		extInterface: extIface ,
		subnetMgr:sm,
	} ,nil
}


func (b *VlanBackend) Run(ctx context.Context){
	<-ctx.Done()
}
// Called when the backend should create or begin managing a new network
func (b *VlanBackend) RegisterNetwork(ctx context.Context, network string, config *subnet.Config) (backend.Network, error){

	cfg :=  struct {
		VlanId  int
		Gateway  string
	}{
		VlanId : 0 ,
	}


	if len(config.Backend) > 0 {
		if err := json.Unmarshal(config.Backend, &cfg); err != nil {
			return nil, fmt.Errorf("error decoding VLAN backend config: %v", err)
		}
	}

	if cfg.VlanId==0 {
		return nil , errors.New("error set vlanid to 0")
	}


	devAttrs := vlanDeviceAttrs{
		vlanId:       uint32(cfg.VlanId),
		name:         fmt.Sprintf("flannel.vlan-%v", cfg.VlanId),
	}

	dev, err := newVLANDevice(&devAttrs)
	if err != nil {
		return nil, err
	}
	subnetAttrs, err := newSubnetAttrs(b.extInterface.ExtAddr, dev.MACAddr())
	if err != nil {
		return nil, err
	}
	lease, err := b.subnetMgr.AcquireLease(ctx, network, subnetAttrs)
	switch err {
	case nil:

	case context.Canceled, context.DeadlineExceeded:
		return nil, err

	default:
		return nil, fmt.Errorf("failed to acquire lease: %v", err)
	}

	// vxlan's subnet is that of the whole overlay network (e.g. /16)
	// and not that of the individual host (e.g. /24)
	vlanNet := ip.IP4Net{
		IP:        lease.Subnet.IP,
		PrefixLen: config.Network.PrefixLen,
	}
	if err = dev.Configure(vlanNet); err != nil {
		return nil, err
	}

	return  newNetwork(network ,b.subnetMgr,b.extInterface , dev , vlanNet,lease)
}
func newSubnetAttrs(publicIP net.IP, mac net.HardwareAddr) (*subnet.LeaseAttrs, error) {
	data, err := json.Marshal(&vlanLeaseAttrs{hardwareAddr(mac)})
	if err != nil {
		return nil, err
	}

	return &subnet.LeaseAttrs{
		PublicIP:    ip.FromIP(publicIP),
		BackendType: "vlan",
		BackendData: json.RawMessage(data),
	}, nil
}
