package flexibleengine

import (
	"fmt"
	"net"

	"github.com/SafeScale/providers/api"
	"github.com/SafeScale/providers/api/IPVersion"
	"github.com/SafeScale/providers/openstack"
	gc "github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/gophercloud/gophercloud/pagination"
)

//VPCRequest defines a request to create a VPC
type VPCRequest struct {
	Name string `json:"name"`
	CIDR string `json:"cidr"`
}

//VPC contains information about a VPC
type VPC struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	CIDR    string `json:"cidr,omitempty"`
	Status  string `json:"status,omitempty"`
	Network *networks.Network
	Router  *routers.Router
}

type vpcCommonResult struct {
	gc.Result
}

// Extract is a function that accepts a result and extracts a Network/VPC from FlexibleEngine response.
func (r vpcCommonResult) Extract() (*VPC, error) {
	var s struct {
		VPC *VPC `json:"vpc"`
	}
	err := r.ExtractInto(&s)
	return s.VPC, err
}

type vpcCreateResult struct {
	vpcCommonResult
}
type vpcGetResult struct {
	vpcCommonResult
}
type vpcDeleteResult struct {
	gc.ErrResult
}

//CreateVPC creates a network, which is managed by VPC in FlexibleEngine
func (client *Client) CreateVPC(req VPCRequest) (*VPC, error) {
	// Only one VPC allowed by client instance
	if client.vpc != nil {
		return nil, fmt.Errorf("failed to create VPC '%s', a VPC named '%s' is already in use", req.Name, client.vpc.Name)
	}

	b, err := gc.BuildRequestBody(req, "vpc")
	if err != nil {
		return nil, fmt.Errorf("Error creating VPC %s: %s", req.Name, errorString(err))
	}

	resp := vpcCreateResult{}
	url := client.Network.Endpoint + "v1/" + client.Opts.ProjectID + "/vpcs"
	opts := gc.RequestOpts{
		JSONBody:     b,
		JSONResponse: &resp.Body,
		OkCodes:      []int{200, 201},
	}
	_, err = client.Provider.Request("POST", url, &opts)
	vpc, err := resp.Extract()
	if err != nil {
		return nil, fmt.Errorf("Error creating VPC %s: %s", req.Name, errorString(err))
	}

	// Searching for the OpenStack Router corresponding to the VPC (router.id == vpc.id)
	router, err := routers.Get(client.Network, vpc.ID).Extract()
	if err != nil {
		client.DeleteVPC(vpc.ID)
		return nil, fmt.Errorf("Error creating VPC %s: %s", req.Name, errorString(err))
	}
	vpc.Router = router

	// Searching for the OpenStack Network corresponding to the VPC (network.name == vpc.id)
	network, err := client.findOpenstackNetworkByName(vpc.ID)
	if err != nil {
		return nil, fmt.Errorf("Error creating VPC %s: %s", req.Name, errorString(err))
	}
	vpc.Network = network

	return vpc, nil
}

func (client *Client) findOpenstackNetworkByName(name string) (*networks.Network, error) {
	// Searching for the network corresponding to the VPC (network.name == vpc.id)
	pager := networks.List(client.Network, networks.ListOpts{
		Name: name,
	})
	var network networks.Network
	found := false
	err := pager.EachPage(func(page pagination.Page) (bool, error) {
		list, err := networks.ExtractNetworks(page)
		if err != nil {
			return false, fmt.Errorf("Error finding Openstack Network named '%s': %s", name, errorString(err))
		}
		for _, n := range list {
			if n.Name == name {
				found = true
				network = n
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to find Openstack Network named '%s': %s", name, errorString(err))
	}
	if found {
		return &network, nil
	}
	return nil, fmt.Errorf("Openstack Network named '%s' not found", name)
}

//GetVPC returns the information about a VPC identified by 'id'
func (client *Client) GetVPC(id string) (*VPC, error) {
	r := vpcGetResult{}
	url := client.Network.Endpoint + "v1/" + client.Opts.ProjectID + "/vpcs/" + id
	opts := gc.RequestOpts{
		JSONResponse: &r.Body,
		OkCodes:      []int{200, 201},
	}
	_, err := client.Provider.Request("GET", url, &opts)
	r.Err = err
	vpc, err := r.Extract()
	if err != nil {
		return nil, fmt.Errorf("Error getting Network %s: %s", id, errorString(err))
	}
	return vpc, nil
}

//ListVPCs lists all the VPC created
func (client *Client) ListVPCs() ([]VPC, error) {
	var vpcList []VPC
	return vpcList, fmt.Errorf("flexibleengine.ListVPCs() not yet implemented")
}

//DeleteVPC deletes a Network (ie a VPC in Flexible Engine) identified by 'id'
func (client *Client) DeleteVPC(id string) error {
	return fmt.Errorf("flexibleengine.DeleteVPC() not implemented yet")
}

//CreateNetwork creates a network (ie a subnet in the network associated to VPC in FlexibleEngine
func (client *Client) CreateNetwork(req api.NetworkRequest) (*api.Network, error) {
	subnet, err := client.findSubnetByName(req.Name)
	if subnet == nil && err != nil {
		return nil, err
	}
	if subnet != nil {
		return nil, fmt.Errorf("Network '%s' already exists", req.Name)
	}

	subnet, err = client.createSubnet(req.Name, req.CIDR)
	if err != nil {
		return nil, fmt.Errorf("Error creating network named '%s': %s", req.Name, errorString(err))
	}

	return &api.Network{
		ID:        subnet.ID,
		Name:      subnet.Name,
		CIDR:      subnet.CIDR,
		IPVersion: fromIntIPVersion(subnet.IPVersion),
	}, nil
}

//GetNetwork returns the network identified by id
func (client *Client) GetNetwork(id string) (*api.Network, error) {
	subnet, err := client.getSubnet(id)
	if err != nil {
		return nil, fmt.Errorf("Error getting network id '%s': %s", id, errorString(err))
	}

	return &api.Network{
		ID:        subnet.ID,
		Name:      subnet.Name,
		CIDR:      subnet.CIDR,
		IPVersion: fromIntIPVersion(subnet.IPVersion),
	}, nil
}

//ListNetworks lists available networks
func (client *Client) ListNetworks() ([]api.Network, error) {
	subnetList, err := client.listSubnets()
	if err != nil {
		return nil, fmt.Errorf("Failed to get networks list: %s", errorString(err))
	}
	var networkList []api.Network
	for _, subnet := range *subnetList {
		networkList = append(networkList, api.Network{
			ID:        subnet.ID,
			Name:      subnet.Name,
			CIDR:      subnet.CIDR,
			IPVersion: fromIntIPVersion(subnet.IPVersion),
		})
	}
	return networkList, nil
}

//DeleteNetwork consists to delete subnet in FlexibleEngine VPC
func (client *Client) DeleteNetwork(id string) error {
	err := client.deleteSubnet(id)
	return err
}

type subnetRequest struct {
	Name             string   `json:"name"`
	CIDR             string   `json:"cidr"`
	GatewayIP        string   `json:"gateway_ip"`
	DHCPEnable       *bool    `json:"dhcp_enable,omitempty"`
	PrimaryDNS       string   `json:"primary_dns,omitempty"`
	SecondaryDNS     string   `json:"secondary_dns,omitempty"`
	DNSList          []string `json:"dnsList,omitempty"`
	AvailabilityZone string   `json:"availability_zone,omitempty"`
	VPCID            string   `json:"vpc_id"`
}

type subnetCommonResult struct {
	gc.Result
}

// Extract is a function that accepts a result and extracts a Subnet from FlexibleEngine response.
func (r subnetCommonResult) Extract() (*subnets.Subnet, error) {
	var s struct {
		Subnet *subnets.Subnet `json:"subnet"`
	}
	err := r.ExtractInto(&s)
	return s.Subnet, err
}

type subnetCreateResult struct {
	subnetCommonResult
}
type subnetGetResult struct {
	subnetCommonResult
}
type subnetDeleteResult struct {
	gc.ErrResult
}

//convertIPv4ToNumber converts a net.IP to a uint32 representation
func convertIPv4ToNumber(IP net.IP) (uint32, error) {
	if IP.To4() == nil {
		return 0, fmt.Errorf("Not an IPv4")
	}
	n := uint32(IP[0])*0x1000000 + uint32(IP[1])*0x10000 + uint32(IP[2])*0x100 + uint32(IP[3])
	return n, nil
}

//convertNumberToIPv4 converts a uint32 representation of an IPv4 Address to net.IP
func convertNumberToIPv4(n uint32) net.IP {
	a := byte(n >> 24)
	b := byte((n & 0xff0000) >> 16)
	c := byte((n & 0xff00) >> 8)
	d := byte(n & 0xff)
	IP := net.IPv4(a, b, c, d)
	return IP
}

//createSubnet creates a subnet using native FlexibleEngine API
func (client *Client) createSubnet(name string, cidr string) (*subnets.Subnet, error) {
	net, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("Failed to choose gateway IP address for the subnet: %s", errorString(err))
	}
	n, err := convertIPv4ToNumber(net.To4())
	if err != nil {
		return nil, fmt.Errorf("Failed to choose gateway IP address for the subnet: %s", errorString(err))
	}
	gw := convertNumberToIPv4(n + 1)

	bYes := true
	req := subnetRequest{
		Name:         name,
		CIDR:         cidr,
		VPCID:        client.vpc.ID,
		DHCPEnable:   &bYes,
		GatewayIP:    gw.String(),
		PrimaryDNS:   "100.125.0.41",
		SecondaryDNS: "100.126.0.41",
		DNSList: []string{
			"100.125.0.41",
			"100.126.0.41",
		},
	}
	b, err := gc.BuildRequestBody(req, "subnet")
	if err != nil {
		return nil, fmt.Errorf("Error preparing Subnet %s creation: %s", req.Name, errorString(err))
	}

	resp := subnetCreateResult{}
	url := client.Network.Endpoint + "v1/" + client.Opts.ProjectID + "/subnets"
	opts := gc.RequestOpts{
		JSONBody:     b,
		JSONResponse: &resp.Body,
		OkCodes:      []int{200, 201},
	}
	_, err = client.Provider.Request("POST", url, &opts)
	if err != nil {
		return nil, fmt.Errorf("Error requesting Subnet %s creation: %s", req.Name, errorString(err))
	}
	subnet, err := resp.Extract()
	if err != nil {
		return nil, fmt.Errorf("Error creating Subnet %s: %s", req.Name, errorString(err))
	}

	return subnet, nil
}

//ListSubnets lists available subnet in VPC
func (client *Client) listSubnets() (*[]subnets.Subnet, error) {
	url := client.Network.Endpoint + "v1/" + client.Opts.ProjectID + "/subnets?vpc_id=" + client.vpc.ID
	pager := pagination.NewPager(client.Network, url, func(r pagination.PageResult) pagination.Page {
		return subnets.SubnetPage{pagination.LinkedPageBase{PageResult: r}}
	})
	var subnetList []subnets.Subnet
	pager.EachPage(func(page pagination.Page) (bool, error) {
		list, err := subnets.ExtractSubnets(page)
		if err != nil {
			return false, fmt.Errorf("Error listing subnets: %s", errorString(err))
		}

		for _, subnet := range list {
			subnetList = append(subnetList, subnet)
		}
		return true, nil
	})
	return &subnetList, nil
}

//getSubnet lists available subnet in VPC
func (client *Client) getSubnet(id string) (*subnets.Subnet, error) {
	r := subnetGetResult{}
	url := client.Network.Endpoint + "v1/" + client.Opts.ProjectID + "/subnets/" + id
	opts := gc.RequestOpts{
		JSONResponse: &r.Body,
		OkCodes:      []int{200, 201},
	}
	_, err := client.Provider.Request("GET", url, &opts)
	r.Err = err
	subnet, err := r.Extract()
	if err != nil {
		return nil, fmt.Errorf("Failed to get information for subnet id '%s': %s", id, errorString(err))
	}
	return subnet, nil
}

//deleteSubnet deletes a subnet
func (client *Client) deleteSubnet(id string) error {
	resp := subnetDeleteResult{}
	url := client.Network.Endpoint + "v1/" + client.Opts.ProjectID + "/vpcs/" + client.vpc.ID + "/subnets/" + id
	opts := gc.RequestOpts{
		//JSONResponse: &resp.Body,
		OkCodes: []int{204},
	}
	_, err := client.Provider.Request("DELETE", url, &opts)
	if err != nil {
		return fmt.Errorf("Error requesting subnet id '%s' deletion: %s", id, errorString(err))
	}
	err = resp.ExtractErr()
	if err != nil {
		return fmt.Errorf("Error deleting subnet id '%s': %s", id, errorString(err))
	}
	return nil
}

/* Not needed ?
//findOpenstackSubnetById returns information about subnet
func (client *Client) findOpenstackSubnetByID(id string) (*subnets.Subnet, error) {
	subnet, err := subnets.Get(client.Network, id).Extract()
	if err != nil {
		return nil, fmt.Errorf("Error finding subnet id '%s': %s", id, errorString(err))
	}
	return subnet, nil
}
*/

//findSubnetByName returns a subnets.Subnet if subnet named as 'name' exists
func (client *Client) findSubnetByName(name string) (*subnets.Subnet, error) {
	subnetList, err := client.listSubnets()
	if err != nil {
		return nil, fmt.Errorf("Failed to find in Subnets: %s", errorString(err))
	}
	found := false
	var subnet subnets.Subnet
	for _, s := range *subnetList {
		if s.Name == name {
			found = true
			subnet = s
			break
		}
	}
	if !found {
		return nil, nil
	}
	return &subnet, nil
}

func fromIntIPVersion(v int) IPVersion.Enum {
	if v == 4 {
		return IPVersion.IPv4
	}
	if v == 6 {
		return IPVersion.IPv6
	}
	return -1
}

/*
 * Invalidating methods from openstack
 */

//CreateSubnet exists only to invalidate code from openstack
// Subnets are managed by xxxNetwork() instead
func (client *Client) CreateSubnet(name string, networkID string, cidr string, ipVersion IPVersion.Enum) (*openstack.Subnet, error) {
	return nil, fmt.Errorf("flexibleengine.CreateSubnet() isn't available by design. Use flexibleengine.CreateNetwork() instead")
}

//GetSubnet exists only to invalidate code from openstack
// Subnets are managed by xxxNetwork() for FlexibleEngine
func (client *Client) GetSubnet(id string) (*openstack.Subnet, error) {
	return nil, fmt.Errorf("flexibleengine.GetSubnet() isn't available by design. Use flexibleengine.GetNetwork() instead")
}

//ListSubnets exists only to invalidate code from openstack
// Subnets are managed by xxxNetwork() for FlexibleEngine
func (client *Client) ListSubnets(netID string) ([]openstack.Subnet, error) {
	var subnetList []openstack.Subnet
	return subnetList, fmt.Errorf("flexibleengine.ListSubnets() isn't available by design. Use flexibleengine.ListNetworks() instead")
}

//DeleteSubnet exists only to invalidate code from openstack.
// Subnets are managed by xxxNetwork() for FlexibleEngine
func (client *Client) DeleteSubnet(id string) error {
	return fmt.Errorf("flexibleengine.DeleteSubnet() isn't available by design. Use flexibleengine.DeleteNetwork() instead")
}

//CreateRouter exists only to invalidate code from openstack.
func (client *Client) CreateRouter(req openstack.RouterRequest) (*openstack.Router, error) {
	return nil, fmt.Errorf("flexibleengine.CreateRouter() isn't available by design")
}

//GetRouter exists only to invalidate code from openstack.
func (client *Client) GetRouter(id string) (*openstack.Router, error) {
	return nil, fmt.Errorf("flexibleengine.GetRouter() isn't available by design")
}

//ListRouter exists only to invalidate code from openstack.
func (client *Client) ListRouter() ([]openstack.Router, error) {
	var ns []openstack.Router
	return ns, fmt.Errorf("flexibleengine.ListRouter() isn't available by design")
}

//DeleteRouter exists only to invalidate code from openstack.
func (client *Client) DeleteRouter(id string) error {
	return fmt.Errorf("flexibleengine.DeleteRouter() isn't available by design")
}

//AddSubnetToRouter exists only to invalidate code from openstack.
func (client *Client) AddSubnetToRouter(routerID string, subnetID string) error {
	return fmt.Errorf("flexibleengine.AddSubnetToRouter() isn't available by design")
}

//RemoveSubnetFromRouter exists only to invalidate code from openstack.
func (client *Client) RemoveSubnetFromRouter(routerID string, subnetID string) error {
	return fmt.Errorf("flexibleengine.RemoveSubnetFromRouter() isn't available by design")
}

//CreateGateway exists only to invalidate code from openstack.
func (client *Client) CreateGateway(req api.GWRequest) error {
	return fmt.Errorf("flexibleengine.CreateGateway() isn't available by design")
}

//DeleteGateway exists only to invalidate code from openstack.
func (client *Client) DeleteGateway(networkID string) error {
	return fmt.Errorf("flexibleengine.DeleteGateway() isn't available by design")
}