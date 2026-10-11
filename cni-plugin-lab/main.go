package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"runtime"
	"syscall"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/utils/buildversion"
	"github.com/vishvananda/netlink"
)

type NetConf struct {
	types.NetConf
	Bridge string `json:"bridge"`
	MTU    int    `json:"mtu"`
}

func init() {
	runtime.LockOSThread()
}

func loadConf(data []byte) (*NetConf, error) {
	conf := &NetConf{Bridge: "cni0", MTU: 1500}
	if err := json.Unmarshal(data, conf); err != nil {
		return nil, fmt.Errorf("decode network configuration: %w", err)
	}
	if conf.IPAM.Type == "" {
		return nil, errors.New("ipam.type is required")
	}
	return conf, nil
}

func ensureBridge(name string, mtu int) (*netlink.Bridge, error) {
	link, err := netlink.LinkByName(name)
	if err == nil {
		bridge, ok := link.(*netlink.Bridge)
		if !ok {
			return nil, fmt.Errorf("%q exists but is not a bridge", name)
		}
		return bridge, netlink.LinkSetUp(bridge)
	}

	attrs := netlink.NewLinkAttrs()
	attrs.Name = name
	attrs.MTU = mtu
	bridge := &netlink.Bridge{LinkAttrs: attrs}
	if err := netlink.LinkAdd(bridge); err != nil && err != syscall.EEXIST {
		return nil, fmt.Errorf("create bridge %q: %w", name, err)
	}
	link, err = netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("find bridge %q: %w", name, err)
	}
	bridge, ok := link.(*netlink.Bridge)
	if !ok {
		return nil, fmt.Errorf("%q is not a bridge", name)
	}
	return bridge, netlink.LinkSetUp(bridge)
}

func ensureGateway(bridge netlink.Link, gateway net.IP, mask net.IPMask) error {
	address := &netlink.Addr{IPNet: &net.IPNet{IP: gateway, Mask: mask}}
	if err := netlink.AddrAdd(bridge, address); err != nil && err != syscall.EEXIST {
		return fmt.Errorf("add gateway %s to %s: %w", gateway, bridge.Attrs().Name, err)
	}
	return ip.EnableIP4Forward()
}

func cmdAdd(args *skel.CmdArgs) error {
	conf, err := loadConf(args.StdinData)
	if err != nil {
		return err
	}
	bridge, err := ensureBridge(conf.Bridge, conf.MTU)
	if err != nil {
		return err
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("open network namespace %q: %w", args.Netns, err)
	}
	defer netns.Close()

	hostNS, err := ns.GetCurrentNS()
	if err != nil {
		return fmt.Errorf("open host network namespace: %w", err)
	}
	defer hostNS.Close()

	var hostInterface, containerInterface *current.Interface
	err = netns.Do(func(_ ns.NetNS) error {
		hostVeth, containerVeth, err := ip.SetupVeth(args.IfName, conf.MTU, "", hostNS)
		if err != nil {
			return err
		}
		hostInterface = &current.Interface{Name: hostVeth.Name, Mac: hostVeth.HardwareAddr.String()}
		containerInterface = &current.Interface{
			Name:    containerVeth.Name,
			Mac:     containerVeth.HardwareAddr.String(),
			Sandbox: args.Netns,
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("create veth pair: %w", err)
	}

	success := false
	defer func() {
		if !success {
			_ = netns.Do(func(_ ns.NetNS) error { return ip.DelLinkByName(args.IfName) })
		}
	}()

	hostVeth, err := netlink.LinkByName(hostInterface.Name)
	if err != nil {
		return fmt.Errorf("find host veth %q: %w", hostInterface.Name, err)
	}
	if err := netlink.LinkSetMaster(hostVeth, bridge); err != nil {
		return fmt.Errorf("attach %s to %s: %w", hostInterface.Name, conf.Bridge, err)
	}

	ipamResult, err := ipam.ExecAdd(conf.IPAM.Type, args.StdinData)
	if err != nil {
		return fmt.Errorf("IPAM ADD: %w", err)
	}
	defer func() {
		if !success {
			_ = ipam.ExecDel(conf.IPAM.Type, args.StdinData)
		}
	}()

	result, err := current.NewResultFromResult(ipamResult)
	if err != nil {
		return fmt.Errorf("convert IPAM result: %w", err)
	}
	if len(result.IPs) == 0 {
		return errors.New("IPAM returned no addresses")
	}

	result.Interfaces = []*current.Interface{
		{Name: bridge.Attrs().Name, Mac: bridge.Attrs().HardwareAddr.String()},
		hostInterface,
		containerInterface,
	}
	for _, address := range result.IPs {
		if address.Address.IP.To4() == nil {
			return errors.New("this tutorial plugin supports IPv4 only")
		}
		address.Interface = current.Int(2)
		if address.Gateway != nil {
			if err := ensureGateway(bridge, address.Gateway, address.Address.Mask); err != nil {
				return err
			}
		}
	}

	if err := netns.Do(func(_ ns.NetNS) error {
		return ipam.ConfigureIface(args.IfName, result)
	}); err != nil {
		return fmt.Errorf("configure %s: %w", args.IfName, err)
	}

	success = true
	return types.PrintResult(result, conf.CNIVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	conf, err := loadConf(args.StdinData)
	if err != nil {
		return err
	}

	if args.Netns != "" {
		err = ns.WithNetNSPath(args.Netns, func(_ ns.NetNS) error {
			err := ip.DelLinkByName(args.IfName)
			if err == ip.ErrLinkNotFound {
				return nil
			}
			return err
		})
		var missing ns.NSPathNotExistErr
		if err != nil && !errors.As(err, &missing) {
			return err
		}
	}
	return ipam.ExecDel(conf.IPAM.Type, args.StdinData)
}

func cmdCheck(args *skel.CmdArgs) error {
	conf, err := loadConf(args.StdinData)
	if err != nil {
		return err
	}
	if err := ipam.ExecCheck(conf.IPAM.Type, args.StdinData); err != nil {
		return err
	}
	if conf.RawPrevResult == nil {
		return errors.New("CHECK requires prevResult")
	}
	if err := version.ParsePrevResult(&conf.NetConf); err != nil {
		return err
	}
	result, err := current.NewResultFromResult(conf.PrevResult)
	if err != nil {
		return err
	}
	if _, err := netlink.LinkByName(conf.Bridge); err != nil {
		return fmt.Errorf("bridge %s is missing: %w", conf.Bridge, err)
	}
	return ns.WithNetNSPath(args.Netns, func(_ ns.NetNS) error {
		if err := ip.ValidateExpectedInterfaceIPs(args.IfName, result.IPs); err != nil {
			return err
		}
		return ip.ValidateExpectedRoute(result.Routes)
	})
}

func cmdStatus(args *skel.CmdArgs) error {
	conf, err := loadConf(args.StdinData)
	if err != nil {
		return err
	}
	return ipam.ExecStatus(conf.IPAM.Type, args.StdinData)
}

func main() {
	skel.PluginMainFuncs(skel.CNIFuncs{
		Add:    cmdAdd,
		Del:    cmdDel,
		Check:  cmdCheck,
		Status: cmdStatus,
	}, version.All, buildversion.BuildString("cni-plugin"))
}
