package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"

	"github.com/idb-project/idbclient"
	"github.com/idb-project/idbclient/machine"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"context"
)

var debug = false

// Properties to retrieve for vms
var props = []string{"name", "capability", "config", "datastore", "environmentBrowser", "guest", "guestHeartbeatStatus", "layout", "layoutEx", "network", "parentVApp", "resourceConfig", "resourcePool", "rootSnapshot", "runtime", "snapshot", "storage", "summary"}

// machineFromVM tries to fill a IDB-machine with values from
// a vsphere virtual machine. The resulting machine should always
// be usable in the IDB, with missing values replaced by sane defaults.
func machineFromVM(ctx context.Context, c *govmomi.Client, vm mo.VirtualMachine, lookup bool, unknownSuffix string, fqdnStrip bool) (*machine.Machine, error) {
	m := new(machine.Machine)

	m.Os, m.OsRelease = osFromVM(vm)
	m.Fqdn = fqdnFromVM(vm, lookup, unknownSuffix, fqdnStrip)

	m.Cores = int(vm.Summary.Config.NumCpu)
	m.RAM = int(vm.Summary.Config.MemorySizeMB)
	m.Diskspace = diskspaceFromVM(vm)

	if vm.Guest != nil {
		m.Nics = nicsFromVM(vm)
	}

	pc := property.DefaultCollector(c.Client)

	host := mo.HostSystem{}
	err := pc.RetrieveOne(ctx, *vm.Summary.Runtime.Host, []string{"name"}, &host)
	if err != nil {
		return nil, err
	}

	if debug {
		log.Println("Found vm host name:", host.Name)
	}
	m.Vmhost = host.Name

	m.DeviceTypeID = machine.DeviceTypeVirtual

	return m, nil
}

// diskspaceFromVM returns the committed space of the guest in bytes.
func diskspaceFromVM(vm mo.VirtualMachine) int {
	if vm.Summary.Storage == nil {
		return 0
	}

	return int((vm.Summary.Storage.Committed))
}

// osFromVM tries to extract os and os release information.
// os is selected from guestId and guestFamily in this order.
// os release is guestFullName.
// See: https://pubs.vmware.com/vsphere-60/topic/com.vmware.wssdk.apiref.doc/vim.vm.GuestInfo.html
func osFromVM(vm mo.VirtualMachine) (string, string) {
	var os string
	var osRelease string

	if vm.Guest == nil {
		if debug {
			log.Println("vm.Guest is nil")
		}
		return "", ""
	}

	switch {
	case vm.Guest.GuestId != "":
		if debug {
			log.Println("using vm.Guest.GuestId as os")
		}
		os = vm.Guest.GuestId
	case vm.Guest.GuestFamily != "":
		if debug {
			log.Println("using vm.Guest.GuestFamily as os")
		}
		os = vm.Guest.GuestFamily
	}

	switch {
	case vm.Guest.GuestFullName != "":
		if debug {
			log.Println("using vm.Guest.GuestFullName as os release")
		}
		osRelease = vm.Guest.GuestFullName
	}

	return os, osRelease
}

// fqdnFromVM tries to extract a hostname from the VM.
// If the VM guest has a hostname set, it is used.
// When no hostname is set, and lookups are enabled,
// a reverse lookup is performed for each IP of the guest.
// If none of this works, an fqdn in the format
// is returned with the prefix "vsphere.unknown." and name of
// the vm concatenated, eg. "vsphere.unknown.vmname".
func fqdnFromVM(vm mo.VirtualMachine, lookup bool, unknownSuffix string, fqdnStrip bool) string {
	var hostName string

	if debug {
		log.Println("trying to find fqdn")
	}

	// This shouldn't happen.
	if vm.Guest == nil {
		if debug {
			log.Println("vm.Guest is nil")
		}

		return fmt.Sprintf("noguest%v", unknownSuffix)
	}

	switch {
	case vm.Guest.HostName != "":
		if debug {
			log.Println("using vm.Guest.HostName as fqdn")
		}

		hostName = vm.Guest.HostName

	case lookup:
		if debug {
			log.Println("trying to reverse-lookup fqdn")
		}

		nics := nicsFromVM(vm)
		for _, nic := range nics {

			if nic.IPAddress.Addr != "" {
				if debug {
					log.Printf("reverse lookup for: %v\n", nic.IPAddress.Addr)
				}

				nicHostName, err := net.LookupAddr(nic.IPAddress.Addr)
				if err != nil || len(nicHostName) < 1 {
					if debug {
						log.Printf("no hostname found for %v", nic.IPAddress.Addr)
					}

					continue
				}

				if debug {
					log.Printf("found hostname %v for %v\n", nicHostName[0], nic.IPAddress.Addr)
				}

				hostName = nicHostName[0]
			}
		}
	}

	if hostName == "" {
		if debug {
			log.Printf("no fqdn found, falling back to: %v\n", vm.Name)
		}

		hostName = vm.Name
	}

	// check for valid fqdn (must contain a dot) and append unknownSuffix if invalid
	if strings.IndexRune(hostName, '.') == -1 {
		if debug {
			log.Println("fqdn doesn't contain '.', appending unknownSuffix")
		}

		hostName = fmt.Sprintf("%v%v", hostName, unknownSuffix)
	}

	if debug {
		log.Printf("using fqdn: %v", hostName)
	}

	// remove all characters which are invalid for fqdns.
	if fqdnStrip {
		hostName = strings.Map(hostNameMap, hostName)

		if debug {
			log.Printf("removed invalid characters from fqdn: %v", hostName)
		}
	}

	return hostName
}

// hostNameMap converts to lowercase and removes everything except
// 'a'-'z', '0'-'9', '.' and '-'
func hostNameMap(r rune) rune {
	switch {
	case r >= 'A' && r <= 'Z':
		r += 'a' - 'A'
		return r
	case r >= 'a' && r <= 'z':
		return r
	case r >= '0' && r <= '9':
		return r
	case r == '.':
		return r
	case r == '-':
		return r
	default:
		return -1
	}
}

// nicsFromVM tries to extract configured network interfaces and their
// ip address and netmask.
func nicsFromVM(vm mo.VirtualMachine) []machine.Nic {
	nics := make([]machine.Nic, 0)

	if vm.Guest == nil {
		if debug {
			log.Println("vm.Guest is nil")
		}

		return nil
	}

	if vm.Guest.Net == nil {
		if debug {
			log.Println("vm.Guest.Net is nil")
		}

		return nil
	}

	for i, vmNic := range vm.Guest.Net {
		if vmNic.IpConfig == nil {
			if debug {
				log.Println("vmNic.IpConfig is nil")
			}

			continue
		}

		if vmNic.IpConfig.IpAddress == nil {
			if debug {
				log.Println("vmNic.IpConfig.IpAddress is nil")
			}

			continue
		}

		for _, addr := range vmNic.IpConfig.IpAddress {
			ipString := addr.IpAddress

			ip := net.ParseIP(ipString)
			if ip == nil {
				if debug {
					log.Printf("%v is not a valid ip address\n", ipString)
				}
				continue
			}

			switch {
			case ip.To4() != nil:
				net, err := suffixToNetmask(addr.PrefixLength)
				if err != nil {
					net = "unknown"
				}

				if debug {
					log.Printf("found nic with v4 address: %v network: %v\n", ipString, net)
				}
				nics = append(nics, machine.Nic{Name: fmt.Sprintf("unknown%v", i), IPAddress: machine.IPAddress{Addr: ipString, Netmask: net}})

			case ip.To16() != nil:
				net := fmt.Sprintf("%d", addr.PrefixLength)

				if debug {
					log.Printf("found nic with v6 address: %v/%v\n", ipString, net)
				}
				nics = append(nics, machine.Nic{Name: fmt.Sprintf("unknown%v", i), IPAddress: machine.IPAddress{AddrV6: ipString, NetmaskV6: net}})

			default:
				continue
			}
		}
	}

	return nics
}

// prefixToNetmask converts a IPv4 suffix to a netmask.
// Eg. 24 will be converted to 255.255.255.0
// An error is returned if the suffix is lower than 0 or larger than 32.
func suffixToNetmask(suffix int32) (string, error) {
	if suffix < 0 || suffix > 32 {
		return "", errors.New(fmt.Sprintf("Invalid suffix: %v", suffix))
	}

	mask := uint32(0xffffffff) >> uint32(32-suffix) << uint32(32-suffix)
	return fmt.Sprintf("%v.%v.%v.%v", mask&0xFF000000>>24, mask&0x00FF0000>>16, mask&0x0000FF00>>8, mask&0xFF), nil
}

func getVMs(ctx context.Context, c *govmomi.Client, props []string) ([]mo.VirtualMachine, error) {
	/*	ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Connect and log in to ESX or vCenter
		c, err := govmomi.NewClient(ctx, u, true)
		if err != nil {
			return nil, err
		}
	*/

	f := find.NewFinder(c.Client, true)

	// Find one and only datacenter
	dc, err := f.DefaultDatacenter(ctx)
	if err != nil {
		return nil, err
	}

	// Make future calls local to this datacenter
	f.SetDatacenter(dc)

	// Find virtual machines in datacenter
	vmList, err := f.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, err
	}

	pc := property.DefaultCollector(c.Client)

	// Convert datastores into list of references
	var refs []types.ManagedObjectReference
	for _, vm := range vmList {
		refs = append(refs, vm.Reference())
	}

	// Retrieve name property for all vms
	var vms []mo.VirtualMachine
	err = pc.Retrieve(ctx, refs, props, &vms)
	if err != nil {
		return nil, err
	}

	return vms, nil

}

func main() {
	c, err := loadConfig(*configFile)
	if err != nil {
		log.Fatal(err)
	}
	debug = c.Debug

	i, err := idbclient.NewIdb(c.IdbUrl, c.IdbToken, c.InsecureSkipVerify)
	if err != nil {
		log.Fatal(err)
	}
	i.Debug = c.Debug

	vmwareUrl, err := url.Parse(c.VmwareUrl)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect and log in to ESX or vCenter
	vsc, err := govmomi.NewClient(ctx, vmwareUrl, true)
	if err != nil {
		log.Fatal(err)
	}

	vms, err := getVMs(ctx, vsc, props)
	if err != nil {
		log.Fatal(err)
	}

	for _, vm := range vms {
		x, err := machineFromVM(ctx, vsc, vm, c.Lookup, c.UnknownSuffix, c.FqdnStrip)
		if err != nil {
			log.Fatal(err)
		}

		if !*dryrun {
			_, err := i.UpdateMachine(x, c.Create)
			if err != nil {
				log.Fatal(err)
			}
		}

		if c.Debug {
			log.Printf("VMware machine:\n%#v\n", vm)
			log.Printf("IDB machine:\n%#v\n", x)
		}
	}
}
