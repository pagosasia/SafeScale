/*
 * Copyright 2018, CS Systemes d'Information, http://www.c-s.fr
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package flexibleengine

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"

	"github.com/CS-SI/SafeScale/providers/model"
	"github.com/CS-SI/SafeScale/providers/model/enums/IPVersion"
	"github.com/CS-SI/SafeScale/providers/openstack"
	"github.com/CS-SI/SafeScale/utils/retry"
	"github.com/CS-SI/SafeScale/utils/retry/Verdict"

	gc "github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/gophercloud/gophercloud/pagination"

	"github.com/pengux/check"
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

// CreateVPC creates a network, which is managed by VPC in FlexibleEngine
func (client *Client) CreateVPC(req VPCRequest) (*VPC, error) {
	// Only one VPC allowed by client instance
	if client.vpc != nil {
		return nil, fmt.Errorf("failed to create VPC '%s', a VPC with this name already exists", req.Name)
	}

	b, err := gc.BuildRequestBody(req, "vpc")
	if err != nil {
		return nil, fmt.Errorf("failed to create VPC '%s': %s", req.Name, openstack.ProviderErrorToString(err))
	}

	resp := vpcCreateResult{}
	url := client.osclt.Network.Endpoint + "v1/" + client.Opts.ProjectID + "/vpcs"
	opts := gc.RequestOpts{
		JSONBody:     b,
		JSONResponse: &resp.Body,
		OkCodes:      []int{200, 201},
	}
	_, err = client.osclt.Provider.Request("POST", url, &opts)
	vpc, err := resp.Extract()
	if err != nil {
		return nil, fmt.Errorf("failed to create VPC '%s': %s", req.Name, openstack.ProviderErrorToString(err))
	}

	// Searching for the OpenStack Router corresponding to the VPC (router.id == vpc.id)
	router, err := routers.Get(client.osclt.Network, vpc.ID).Extract()
	if err != nil {
		nerr := client.DeleteVPC(vpc.ID)
		if nerr != nil {
			log.Warnf("Error deleting VPC: %v", nerr)
		}
		return nil, fmt.Errorf("failed to create VPC '%s': %s", req.Name, openstack.ProviderErrorToString(err))
	}
	vpc.Router = router

	// Searching for the Network binded to the VPC
	network, err := client.findVPCBindedNetwork(vpc.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create VPC '%s': %s", req.Name, openstack.ProviderErrorToString(err))
	}
	vpc.Network = network

	return vpc, nil
}

func (client *Client) findVPCBindedNetwork(vpcName string) (*networks.Network, error) {
	var router *openstack.Router
	found := false
	routers, err := client.osclt.ListRouters()
	if err != nil {
		return nil, fmt.Errorf("failed to list routers: %s", openstack.ProviderErrorToString(err))
	}
	for _, r := range routers {
		if r.Name == vpcName {
			found = true
			router = &r
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("failed to find router associated to VPC '%s'", vpcName)
	}

	network, err := networks.Get(client.osclt.Network, router.NetworkID).Extract()
	if err != nil {
		return nil, fmt.Errorf("failed to find binded network of VPC '%s': %s", vpcName, openstack.ProviderErrorToString(err))
	}
	return network, nil
}

// GetVPC returns the information about a VPC identified by 'id'
func (client *Client) GetVPC(id string) (*VPC, error) {
	r := vpcGetResult{}
	url := client.osclt.Network.Endpoint + "v1/" + client.Opts.ProjectID + "/vpcs/" + id
	opts := gc.RequestOpts{
		JSONResponse: &r.Body,
		OkCodes:      []int{200, 201},
	}
	_, err := client.osclt.Provider.Request("GET", url, &opts)
	r.Err = err
	vpc, err := r.Extract()
	if err != nil {
		return nil, fmt.Errorf("Error getting Network %s: %s", id, openstack.ProviderErrorToString(err))
	}
	return vpc, nil
}

// ListVPCs lists all the VPC created
func (client *Client) ListVPCs() ([]VPC, error) {
	var vpcList []VPC
	return vpcList, fmt.Errorf("flexibleengine.ListVPCs() not yet implemented")
}

// DeleteVPC deletes a Network (ie a VPC in Flexible Engine) identified by 'id'
func (client *Client) DeleteVPC(id string) error {
	return fmt.Errorf("flexibleengine.DeleteVPC() not implemented yet")
}

// CreateNetwork creates a network (ie a subnet in the network associated to VPC in FlexibleEngine
func (client *Client) CreateNetwork(req model.NetworkRequest) (*model.Network, error) {
	log.Debugf("providers.flexibleengine.CreateNetwork(%s) called\n", req.Name)
	subnet, err := client.findSubnetByName(req.Name)
	if subnet == nil && err != nil {
		return nil, err
	}
	if subnet != nil {
		return nil, fmt.Errorf("network '%s' already exists", req.Name)
	}

	if ok, err := validateNetworkName(req); !ok {
		return nil, fmt.Errorf("network name '%s' invalid: %s", req.Name, err)
	}

	// Checks if CIDR is valid...
	_, vpcnetDesc, _ := net.ParseCIDR(client.vpc.CIDR)
	_, networkDesc, err := net.ParseCIDR(req.CIDR)
	if err != nil {
		return nil, fmt.Errorf("failed to create subnet '%s (%s)': %s", req.Name, req.CIDR, err.Error())
	}
	// .. and if CIDR is inside VPC's one
	if !cidrIntersects(vpcnetDesc, networkDesc) {
		return nil, fmt.Errorf("can't create subnet with CIDR '%s': not inside VPC CIDR '%s'", req.CIDR, client.vpc.CIDR)
	}

	// Creates the subnet
	subnet, err = client.createSubnet(req.Name, req.CIDR)
	if err != nil {
		return nil, fmt.Errorf("error creating network '%s': %s", req.Name, openstack.ProviderErrorToString(err))
	}

	defer func() {
		if err != nil {
			derr := client.deleteSubnet(subnet.ID)
			if derr != nil {
				log.Errorf("failed to delete subnet '%s': %v", subnet.Name, derr)
			}
		}
	}()

	network := model.NewNetwork()
	network.ID = subnet.ID
	network.Name = subnet.Name
	network.CIDR = subnet.CIDR
	network.IPVersion = fromIntIPVersion(subnet.IPVersion)

	return network, nil
}

// validateNetworkName validates the name of a Network based on known FlexibleEngine requirements
func validateNetworkName(req model.NetworkRequest) (bool, error) {
	s := check.Struct{
		"Name": check.Composite{
			check.NonEmpty{},
			check.Regex{Constraint: `^[a-zA-Z0-9_-]+$`},
			check.MaxChar{Constraint: 64},
		},
	}

	e := s.Validate(req)
	if e.HasErrors() {
		errors, _ := e.GetErrorsByKey("Name")
		var errs []string
		for _, msg := range errors {
			errs = append(errs, msg.Error())
		}
		return false, fmt.Errorf(strings.Join(errs, "; "))
	}
	return true, nil
}

// GetNetworkByName ...
func (client *Client) GetNetworkByName(name string) (*model.Network, error) {
	if name == "" {
		panic("name is empty!")
	}

	// Gophercloud doesn't propose the way to get a host by name, but OpenStack knows how to do it...
	r := networks.GetResult{}
	_, r.Err = client.osclt.Compute.Get(client.osclt.Network.ServiceURL("subnets?name="+name), &r.Body, &gc.RequestOpts{
		OkCodes: []int{200, 203},
	})
	if r.Err != nil {
		return nil, fmt.Errorf("query for network '%s' failed: %v", name, r.Err)
	}
	subnets, found := r.Body.(map[string]interface{})["subnets"].([]interface{})
	if found && len(subnets) > 0 {
		entry := subnets[0].(map[string]interface{})
		id := entry["id"].(string)
		return client.GetNetwork(id)
	}
	return nil, model.ResourceNotFoundError("network", name)
}

// GetNetwork returns the network identified by id
func (client *Client) GetNetwork(id string) (*model.Network, error) {
	subnet, err := client.getSubnet(id)
	if err != nil {
		spew.Dump(err)
		if !strings.Contains(err.Error(), id) {
			return nil, fmt.Errorf("failed getting network id '%s': %s", id, openstack.ProviderErrorToString(err))
		}
	}
	if subnet == nil || subnet.ID == "" {
		return nil, nil
	}

	net := model.NewNetwork()
	net.ID = subnet.ID
	net.Name = subnet.Name
	net.CIDR = subnet.CIDR
	net.IPVersion = fromIntIPVersion(subnet.IPVersion)
	return net, nil
}

// ListNetworks lists networks
func (client *Client) ListNetworks() ([]*model.Network, error) {
	subnetList, err := client.listSubnets()
	if err != nil {
		return nil, fmt.Errorf("Failed to get networks list: %s", openstack.ProviderErrorToString(err))
	}
	var networkList []*model.Network
	for _, subnet := range *subnetList {
		net := model.NewNetwork()
		net.ID = subnet.ID
		net.Name = subnet.Name
		net.CIDR = subnet.CIDR
		net.IPVersion = fromIntIPVersion(subnet.IPVersion)
		networkList = append(networkList, net)
	}
	return networkList, nil
}

// DeleteNetwork consists to delete subnet in FlexibleEngine VPC
func (client *Client) DeleteNetwork(id string) error {
	return client.deleteSubnet(id)
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

type subnetEx struct {
	subnets.Subnet
	Status string `json:"status"`
}

// Extract is a function that accepts a result and extracts a Subnet from FlexibleEngine response.
func (r subnetCommonResult) Extract() (*subnetEx, error) {
	var s struct {
		//		Subnet *subnets.Subnet `json:"subnet"`
		Subnet *subnetEx `json:"subnet"`
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

// convertIPv4ToNumber converts a net.IP to a uint32 representation
func convertIPv4ToNumber(IP net.IP) (uint32, error) {
	if IP.To4() == nil {
		return 0, fmt.Errorf("Not an IPv4")
	}
	n := uint32(IP[0])*0x1000000 + uint32(IP[1])*0x10000 + uint32(IP[2])*0x100 + uint32(IP[3])
	return n, nil
}

// convertNumberToIPv4 converts a uint32 representation of an IPv4 Address to net.IP
func convertNumberToIPv4(n uint32) net.IP {
	a := byte(n >> 24)
	b := byte((n & 0xff0000) >> 16)
	c := byte((n & 0xff00) >> 8)
	d := byte(n & 0xff)
	IP := net.IPv4(a, b, c, d)
	return IP
}

// cidrIntersects tells if the 2 CIDR passed as parameter intersect
func cidrIntersects(n1, n2 *net.IPNet) bool {
	return n2.Contains(n1.IP) || n1.Contains(n2.IP)
}

// createSubnet creates a subnet using native FlexibleEngine API
func (client *Client) createSubnet(name string, cidr string) (*subnets.Subnet, error) {
	network, networkDesc, _ := net.ParseCIDR(cidr)

	// Validates CIDR regarding the existing subnets
	subnets, err := client.listSubnets()
	if err != nil {
		return nil, err
	}
	for _, s := range *subnets {
		_, sDesc, _ := net.ParseCIDR(s.CIDR)
		if cidrIntersects(networkDesc, sDesc) {
			return nil, fmt.Errorf("can't create subnet '%s (%s)', would intersect with '%s (%s)'", name, cidr, s.Name, s.CIDR)
		}
	}

	// Calculate IP address for gateway
	n, err := convertIPv4ToNumber(network.To4())
	if err != nil {
		return nil, fmt.Errorf("failed to choose gateway IP address for the subnet: %s", openstack.ProviderErrorToString(err))
	}
	gw := convertNumberToIPv4(n + 1)

	dnsList := client.osclt.Cfg.DNSList
	if len(dnsList) == 0 {
		dnsList = []string{"1.1.1.1"}
	}
	var (
		primaryDNS   string
		secondaryDNS string
	)
	if len(dnsList) >= 1 {
		primaryDNS = dnsList[0]
	}
	if len(dnsList) >= 2 {
		secondaryDNS = dnsList[1]
	}
	bYes := true
	req := subnetRequest{
		Name:         name,
		CIDR:         cidr,
		VPCID:        client.vpc.ID,
		DHCPEnable:   &bYes,
		GatewayIP:    gw.String(),
		PrimaryDNS:   primaryDNS,
		SecondaryDNS: secondaryDNS,
		DNSList:      dnsList,
	}
	b, err := gc.BuildRequestBody(req, "subnet")
	if err != nil {
		return nil, fmt.Errorf("error preparing Subnet %s creation: %s", req.Name, openstack.ProviderErrorToString(err))
	}

	respCreate := subnetCreateResult{}
	url := fmt.Sprintf("%sv1/%s/subnets", client.osclt.Network.Endpoint, client.Opts.ProjectID)
	opts := gc.RequestOpts{
		JSONBody:     b,
		JSONResponse: &respCreate.Body,
		OkCodes:      []int{200, 201},
	}
	_, err = client.osclt.Provider.Request("POST", url, &opts)
	if err != nil {
		return nil, fmt.Errorf("error requesting Subnet %s creation: %s", req.Name, openstack.ProviderErrorToString(err))
	}
	subnet, err := respCreate.Extract()
	if err != nil {
		return nil, fmt.Errorf("error creating Subnet %s: %s", req.Name, openstack.ProviderErrorToString(err))
	}

	// Subnet creation started, need to wait the subnet to reach the status ACTIVE
	respGet := subnetGetResult{}
	opts.JSONResponse = &respGet.Body
	opts.JSONBody = nil

	retryErr := retry.WhileUnsuccessfulDelay1SecondWithNotify(
		func() error {
			_, err = client.osclt.Provider.Request("GET", fmt.Sprintf("%s/%s", url, subnet.ID), &opts)
			if err == nil {
				subnet, err = respGet.Extract()
				if err == nil && subnet.Status == "ACTIVE" {
					return nil
				}
			}
			return err
		},
		time.Minute,
		func(try retry.Try, verdict Verdict.Enum) {
			if verdict != Verdict.Done {
				log.Printf("Network '%s' is not in 'ACTIVE' state, retrying...", name)
			}
		},
	)
	return &subnet.Subnet, retryErr
}

// ListSubnets lists available subnet in VPC
func (client *Client) listSubnets() (*[]subnets.Subnet, error) {
	url := client.osclt.Network.Endpoint + "v1/" + client.Opts.ProjectID + "/subnets?vpc_id=" + client.vpc.ID
	pager := pagination.NewPager(client.osclt.Network, url, func(r pagination.PageResult) pagination.Page {
		return subnets.SubnetPage{LinkedPageBase: pagination.LinkedPageBase{PageResult: r}}
	})
	var subnetList []subnets.Subnet
	paginationErr := pager.EachPage(func(page pagination.Page) (bool, error) {
		list, err := subnets.ExtractSubnets(page)
		if err != nil {
			return false, fmt.Errorf("Error listing subnets: %s", openstack.ProviderErrorToString(err))
		}

		for _, subnet := range list {
			subnetList = append(subnetList, subnet)
		}
		return true, nil
	})

	// TODO previously we ignored the error here, consider returning nil, paginationErr
	if paginationErr != nil {
		log.Warnf("We have a pagination error: %v", paginationErr)
	}

	return &subnetList, nil
}

// getSubnet lists available subnet in VPC
func (client *Client) getSubnet(id string) (*subnets.Subnet, error) {
	r := subnetGetResult{}
	url := client.osclt.Network.Endpoint + "v1/" + client.Opts.ProjectID + "/subnets/" + id
	opts := gc.RequestOpts{
		JSONResponse: &r.Body,
		OkCodes:      []int{200, 201},
	}
	_, err := client.osclt.Provider.Request("GET", url, &opts)
	r.Err = err
	subnet, err := r.Extract()
	if err != nil {
		return nil, fmt.Errorf("Failed to get information for subnet id '%s': %s", id, openstack.ProviderErrorToString(err))
	}
	return &subnet.Subnet, nil
}

// deleteSubnet deletes a subnet
func (client *Client) deleteSubnet(id string) error {
	resp := subnetDeleteResult{}
	url := client.osclt.Network.Endpoint + "v1/" + client.Opts.ProjectID + "/vpcs/" + client.vpc.ID + "/subnets/" + id
	opts := gc.RequestOpts{
		OkCodes: []int{204},
	}

	// FlexibleEngine has the curious behavior to be able to tell us all Hosts are deleted, but
	// can't delete the subnet because there is still at least one host...
	// So we retry subnet deletion until all hosts are really deleted and subnet can be deleted
	err := retry.Action(
		func() error {
			r, _ := client.osclt.Provider.Request("DELETE", url, &opts)
			if r.StatusCode == 204 || r.StatusCode == 404 {
				return nil
			}
			return fmt.Errorf("%d", r.StatusCode)
		},
		retry.PrevailDone(retry.Unsuccessful(), retry.Timeout(time.Minute*5)),
		retry.Constant(time.Second*3),
		nil, nil,
		func(t retry.Try, verdict Verdict.Enum) {
			if t.Err != nil {
				switch t.Err.Error() {
				case "409":
					log.Debugln("network still owns host(s), retrying in 3s...")
				default:
					log.Debugf("error submitting network deletion (status=%s), retrying in 3s...", t.Err.Error())
				}
			}
		},
	)
	if err != nil {
		return fmt.Errorf("failed to submit deletion of subnet id '%s': '%s", id, err.Error())
	}
	// Deletion submit has been executed, checking returned error code
	err = resp.ExtractErr()
	if err != nil {
		return fmt.Errorf("Error deleting subnet id '%s': %s", id, openstack.ProviderErrorToString(err))
	}
	return nil
}

// findSubnetByName returns a subnets.Subnet if subnet named as 'name' exists
func (client *Client) findSubnetByName(name string) (*subnets.Subnet, error) {
	subnetList, err := client.listSubnets()
	if err != nil {
		return nil, fmt.Errorf("Failed to find in Subnets: %s", openstack.ProviderErrorToString(err))
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

// CreateGateway creates a gateway for a network.
// By current implementation, only one gateway can exist by Network because the object is intended
// to contain only one hostID
func (client *Client) CreateGateway(req model.GatewayRequest) (*model.Host, error) {
	if req.Network == nil {
		panic("req.Network is nil!")
	}
	gwname := req.Name
	if gwname == "" {
		gwname = "gw-" + req.Network.Name
	}
	hostReq := model.HostRequest{
		ImageID:      req.ImageID,
		KeyPair:      req.KeyPair,
		ResourceName: gwname,
		TemplateID:   req.TemplateID,
		Networks:     []*model.Network{req.Network},
		PublicIP:     true,
	}
	host, err := client.CreateHost(hostReq)
	if err != nil {
		switch err.(type) {
		case model.ErrResourceInvalidRequest:
			return nil, err
		default:
			return nil, fmt.Errorf("Error creating gateway : %s", openstack.ProviderErrorToString(err))
		}
	}
	return host, err
}

// DeleteGateway deletes the gateway associated with network identified by ID
func (client *Client) DeleteGateway(id string) error {
	return client.DeleteHost(id)
}
