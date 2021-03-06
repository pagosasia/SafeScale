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

package services

import (
	"fmt"
	"os"
	"os/user"
	"time"

	"github.com/CS-SI/SafeScale/providers/model/enums/HostState"
	"github.com/CS-SI/SafeScale/providers/model/enums/NetworkProperty"
	"github.com/davecgh/go-spew/spew"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"

	"github.com/CS-SI/SafeScale/broker/client"
	brokerutils "github.com/CS-SI/SafeScale/broker/utils"
	"github.com/CS-SI/SafeScale/providers"
	"github.com/CS-SI/SafeScale/providers/metadata"
	"github.com/CS-SI/SafeScale/providers/model"
	"github.com/CS-SI/SafeScale/providers/model/enums/HostProperty"
	"github.com/CS-SI/SafeScale/providers/model/enums/IPVersion"
	propsv1 "github.com/CS-SI/SafeScale/providers/model/properties/v1"
	"github.com/CS-SI/SafeScale/system"
	"github.com/CS-SI/SafeScale/utils"
	"github.com/CS-SI/SafeScale/utils/retry"
)

//go:generate mockgen -destination=../mocks/mock_hostapi.go -package=mocks github.com/CS-SI/SafeScale/broker/server/services HostAPI

// TODO At service level, we need to log before returning, because it's the last chance to track the real issue in server side

// HostAPI defines API to manipulate hosts
type HostAPI interface {
	Create(name string, net string, cpu int, ram float32, disk int, os string, public bool, gpuNumber int, freq float32, force bool) (*model.Host, error)
	List(all bool) ([]*model.Host, error)
	Get(ref string) (*model.Host, error)
	Delete(ref string) error
	SSH(ref string) (*system.SSHConfig, error)
	Reboot(ref string) error
	Start(ref string) error
	Stop(ref string) error
}

// HostService host service
type HostService struct {
	provider *providers.Service
}

// NewHostService ...
func NewHostService(api *providers.Service) HostAPI {
	return &HostService{
		provider: api,
	}
}

// Start starts a host
func (svc *HostService) Start(ref string) error {
	log.Debugf("server.services.HostService.Start(%s) called", ref)
	defer log.Debugf("server.services.HostService.Start(%s) done", ref)

	mh, err := metadata.LoadHost(svc.provider, ref)
	if err != nil {
		// TODO Introduce error level as parameter
		return infraErrf(err, "Error getting ssh config of host '%s': loading host metadata", ref)
	}
	if mh == nil {
		return infraErr(fmt.Errorf("host '%s' not found", ref))
	}
	id := mh.Get().ID
	err = svc.provider.StartHost(id)
	if err != nil {
		return infraErr(err)
	}
	return infraErr(svc.provider.WaitHostState(id, HostState.STARTED, brokerutils.TimeoutCtxHost))
}

// Stop stops a host
func (svc *HostService) Stop(ref string) error {
	log.Debugf("server.services.HostService.Stop(%s) called", ref)
	defer log.Debugf("server.services.HostService.Stop(%s) done", ref)

	mh, err := metadata.LoadHost(svc.provider, ref)
	if err != nil {
		// TODO Introduce error level as parameter
		return infraErrf(err, "Error getting ssh config of host '%s': loading host metadata", ref)
	}
	if mh == nil {
		return infraErr(fmt.Errorf("host '%s' not found", ref))
	}
	id := mh.Get().ID
	err = svc.provider.StopHost(id)
	if err != nil {
		return infraErr(err)
	}
	return infraErr(svc.provider.WaitHostState(id, HostState.STOPPED, brokerutils.TimeoutCtxHost))
}

// Reboot reboots a host
func (svc *HostService) Reboot(ref string) error {
	log.Debugf("server.services.HostService.Reboot(%s) called", ref)
	defer log.Debugf("server.services.HostService.Reboot(%s) done", ref)

	mh, err := metadata.LoadHost(svc.provider, ref)
	if err != nil {
		return infraErr(fmt.Errorf("failed to load metadata of host '%s': %v", ref, err))
	}
	if mh == nil {
		return infraErr(fmt.Errorf("host '%s' not found", ref))
	}
	id := mh.Get().ID
	err = svc.provider.RebootHost(id)
	if err != nil {
		return infraErr(err)
	}
	err = retry.WhileUnsuccessfulDelay5Seconds(
		func() error {
			return svc.provider.WaitHostState(id, HostState.STARTED, brokerutils.TimeoutCtxHost)
		},
		5*time.Minute,
	)
	if err != nil {
		return infraErrf(err, "timeout waiting host '%s' to be rebooted", ref)
	}
	return nil
}

// Create creates a host
func (svc *HostService) Create(
	name string, net string, cpu int, ram float32, disk int, los string, public bool, gpuNumber int, freq float32, force bool,
) (*model.Host, error) {

	log.Debugf("broker.server.services.HostService.Create('%s') called", name)
	defer log.Debugf("broker.server.services.HostService.Create('%s') done", name)

	host, err := svc.provider.GetHostByName(name)
	if err != nil {
		switch err.(type) {
		case model.ErrResourceNotFound:
		default:
			return nil, infraErrf(err, "failure creating host: failed to check if host resource name '%s' is already used: %v", name, err)
		}
	} else {
		return nil, logicErr(fmt.Errorf("failed to create host '%s': name is already used", name))
	}

	networks := []*model.Network{}
	var gw *model.Host
	if len(net) != 0 {
		networkSvc := NewNetworkService(svc.provider)
		n, err := networkSvc.Get(net)
		if err != nil {
			switch err.(type) {
			case model.ErrResourceNotFound:
				return nil, infraErr(err)
			default:
				return nil, infraErrf(err, "Failed to get network resource data: '%s'.", net)
			}
		}
		if n == nil {
			return nil, logicErr(fmt.Errorf("Failed to find network '%s'", net))
		}
		networks = append(networks, n)
		mgw, err := metadata.LoadHost(svc.provider, n.GatewayID)
		if err != nil {
			return nil, infraErr(err)
		}
		if mgw == nil {
			return nil, logicErr(fmt.Errorf("failed to find gateway of network '%s'", net))
		}
		gw = mgw.Get()
	} else {
		net, err := svc.getOrCreateDefaultNetwork()
		if err != nil {
			return nil, infraErr(err)
		}
		networks = append(networks, net)
	}

	templates, err := svc.provider.SelectTemplatesBySize(
		model.SizingRequirements{
			MinCores:    cpu,
			MinRAMSize:  ram,
			MinDiskSize: disk,
			MinGPU:      gpuNumber,
			MinFreq:     freq,
		}, force)
	if err != nil {
		return nil, infraErrf(err, "failed to find template corresponding to requested resources")
	}
	var template model.HostTemplate
	if len(templates) > 0 {
		template = templates[0]
		log.Debugf("Selected template: '%s' (%d core%s, %.01f GB RAM, %d GB disk)", template.Name, template.Cores, utils.Plural(template.Cores), template.RAMSize, template.DiskSize)
	}

	img, err := svc.provider.SearchImage(los)
	if err != nil {
		return nil, infraErr(errors.Wrap(err, "Failed to find image to use on compute resource."))
	}
	hostRequest := model.HostRequest{
		ImageID:        img.ID,
		ResourceName:   name,
		TemplateID:     template.ID,
		PublicIP:       public,
		Networks:       networks,
		DefaultGateway: gw,
	}

	host, err = svc.provider.CreateHost(hostRequest)
	if err != nil {
		switch err.(type) {
		case model.ErrResourceInvalidRequest:
			return nil, infraErr(err)
		default:
			return nil, infraErrf(err, "failed to create compute resource '%s'", hostRequest.ResourceName)
		}
	}

	defer func() {
		if err != nil {
			derr := svc.provider.DeleteHost(host.ID)
			if derr != nil {
				log.Errorf("Failed to delete host '%s': %v", host.Name, derr)
			}
		}
	}()

	// Updates property propsv1.HostSizing
	hostSizingV1 := propsv1.NewHostSizing()
	err = host.Properties.Get(HostProperty.SizingV1, hostSizingV1)
	if err != nil {
		return nil, infraErr(err)
	}
	hostSizingV1.Template = hostRequest.TemplateID
	hostSizingV1.RequestedSize = &propsv1.HostSize{
		Cores:     cpu,
		RAMSize:   ram,
		DiskSize:  disk,
		GPUNumber: gpuNumber,
		CPUFreq:   freq,
	}
	err = host.Properties.Set(HostProperty.SizingV1, hostSizingV1)
	if err != nil {
		return nil, infraErr(err)
	}

	// Sets host extension DescriptionV1
	creator := ""
	hostname, _ := os.Hostname()
	if curUser, err := user.Current(); err == nil {
		creator := curUser.Username
		if hostname != "" {
			creator += "@" + hostname
		}
		if curUser.Name != "" {
			creator += " (" + curUser.Name + ")"
		}
	} else {
		creator = "unknown@" + hostname
	}
	err = host.Properties.Set(string(HostProperty.DescriptionV1), &propsv1.HostDescription{
		Created: time.Now(),
		Creator: creator,
	})

	// Updates host property propsv1.HostNetwork
	hostNetworkV1 := propsv1.NewHostNetwork()
	err = host.Properties.Get(HostProperty.NetworkV1, hostNetworkV1)
	if err != nil {
		return nil, infraErr(err)
	}
	defaultNetworkID := hostNetworkV1.DefaultNetworkID // set earlier by svc.provider.CreateHost()
	gatewayID := ""
	if !public {
		if len(networks) > 0 {
			mgw, err := metadata.LoadGateway(svc.provider, defaultNetworkID)
			if err == nil {
				gatewayID = mgw.Get().ID
			}
		}
	}
	hostNetworkV1.DefaultGatewayID = gatewayID
	err = host.Properties.Set(HostProperty.NetworkV1, hostNetworkV1)
	if err != nil {
		return nil, infraErr(err)
	}
	if net != "" {
		mn, err := metadata.LoadNetwork(svc.provider, net)
		if err != nil {
			return nil, infraErr(err)
		}
		if mn == nil {
			return nil, logicErr(fmt.Errorf("failed to load metadata of network '%s'", net))
		}
		network := mn.Get()
		hostNetworkV1.NetworksByID[network.ID] = network.Name
		hostNetworkV1.NetworksByName[network.Name] = network.ID
	}

	// Updates metadata
	err = metadata.NewHost(svc.provider).Carry(host).Write()
	if err != nil {
		return nil, infraErrf(err, "Metadata creation failed")
	}
	log.Infof("Compute resource created: '%s'", host.Name)

	networkHostsV1 := propsv1.NewNetworkHosts()
	for _, i := range networks {
		err = i.Properties.Get(NetworkProperty.HostsV1, networkHostsV1)
		if err != nil {
			log.Errorf(err.Error())
			continue
		}
		networkHostsV1.ByName[host.Name] = host.ID
		networkHostsV1.ByID[host.ID] = host.Name
		err = i.Properties.Set(NetworkProperty.HostsV1, networkHostsV1)
		if err != nil {
			log.Errorf(err.Error())
		}

		err = metadata.SaveNetwork(svc.provider, i)
		if err != nil {
			log.Errorf(err.Error())
		}
	}

	// A host claimed ready by a Cloud provider is not necessarily ready
	// to be used until ssh service is up and running. So we wait for it before
	// claiming host is created
	log.Infof("Waiting start of SSH service on remote host '%s' ...", host.Name)
	sshSvc := NewSSHService(svc.provider)
	sshCfg, err := sshSvc.GetConfig(host.ID)
	if err != nil {
		return nil, infraErr(err)
	}
	err = sshCfg.WaitServerReady(brokerutils.TimeoutCtxHost)
	if err != nil {
		return nil, infraErr(err)
	}
	if client.IsTimeout(err) {
		return nil, infraErrf(err, "Timeout creating a host")
	}
	log.Infof("SSH service started on host '%s'.", host.Name)

	return host, nil
}

// getOrCreateDefaultNetwork gets network model.SingleHostNetworkName or create it if necessary
// We don't want metadata on this network, so we use directly provider api instead of services
func (svc *HostService) getOrCreateDefaultNetwork() (*model.Network, error) {
	network, err := svc.provider.GetNetworkByName(model.SingleHostNetworkName)
	if err != nil {
		switch err.(type) {
		case model.ErrResourceNotFound:
		default:
			return nil, infraErr(err)
		}
	}
	if network != nil {
		return network, nil
	}

	request := model.NetworkRequest{
		Name:      model.SingleHostNetworkName,
		IPVersion: IPVersion.IPv4,
		CIDR:      "10.0.0.0/8",
	}

	mnet, err := svc.provider.CreateNetwork(request)
	return mnet, infraErr(err)
}

// List returns the host list
func (svc *HostService) List(all bool) ([]*model.Host, error) {
	if all {
		return svc.provider.ListHosts()
	}

	var hosts []*model.Host
	m := metadata.NewHost(svc.provider)
	err := m.Browse(func(host *model.Host) error {
		hosts = append(hosts, host)
		return nil
	})
	if err != nil {
		return hosts, infraErrf(err, "Error listing monitored hosts: browse")
	}
	return hosts, nil
}

// Get returns the host identified by ref, ref can be the name or the id
func (svc *HostService) Get(ref string) (*model.Host, error) {
	// Uses metadata to recover Host id
	mh := metadata.NewHost(svc.provider)
	found, err := mh.ReadByID(ref)
	if err != nil {
		return nil, infraErr(err)
	}
	if !found {
		found, err = mh.ReadByName(ref)
		if err != nil {
			return nil, infraErr(err)
		}
	}
	if !found {
		return nil, logicErr(fmt.Errorf("Cannot find host metadata : host '%s' not found", ref))
	}
	host := mh.Get()
	return svc.provider.GetHost(host)
}

// Delete deletes host referenced by ref
func (svc *HostService) Delete(ref string) error {
	mh := metadata.NewHost(svc.provider)
	found, err := mh.ReadByID(ref)
	if err != nil {
		return infraErrf(err, "can't delete host '%s'", ref)
	}
	if !found {
		found, err = mh.ReadByName(ref)
		if err != nil {
			return infraErrf(err, "can't delete host '%s'", ref)
		}
	}

	if found {
		host := mh.Get()

		// Don't remove a host having shares
		hostSharesV1 := propsv1.NewHostShares()
		err := host.Properties.Get(HostProperty.SharesV1, hostSharesV1)
		if err != nil {
			return infraErrf(err ,"can't delete host '%s'", ref)
		}
		nShares := len(hostSharesV1.ByID)
		if nShares > 0 {
			return logicErr(fmt.Errorf("can't delete host, exports %d share%s", nShares, utils.Plural(nShares)))
		}

		// Don't remove a host with volumes attached
		hostVolumesV1 := propsv1.NewHostVolumes()
		err = host.Properties.Get(HostProperty.VolumesV1, hostVolumesV1)
		if err != nil {
			return infraErr(err)
		}
		nAttached := len(hostVolumesV1.VolumesByID)
		if nAttached > 0 {
			return logicErr(fmt.Errorf("host has %d volume%s attached", nAttached, utils.Plural(nAttached)))
		}

		// Don't remove a host that is a gateway
		hostNetworkV1 := propsv1.NewHostNetwork()
		err = host.Properties.Get(HostProperty.NetworkV1, hostNetworkV1)
		if err != nil {
			return infraErr(err)
		}
		if hostNetworkV1.IsGateway {
			return logicErr(fmt.Errorf("can't delete host, it's a gateway that can't be deleted but with its network"))
		}

		// If host mounted shares, unmounts them before anything else
		hostMountsV1 := propsv1.NewHostMounts()
		err = host.Properties.Get(HostProperty.MountsV1, hostMountsV1)
		if err != nil {
			return infraErr(err)
		}
		shareSvc := NewShareService(svc.provider)
		for _, i := range hostMountsV1.RemoteMountsByPath {
			// Gets share data
			_, share, _, err := shareSvc.Inspect(i.ShareID)
			if err != nil {
				return infraErr(err)
			}

			// Unmounts share from host
			err = shareSvc.Unmount(share.Name, host.Name)
			if err != nil {
				return infraErr(err)
			}
		}

		// Conditions are met, delete host
		err = svc.provider.DeleteHost(host.ID)
		if err != nil {
			spew.Dump(err)
			switch err.(type) {

			}
			log.Errorf("Failed to delete host: %v", err)
		}

		// Update networks property prosv1.NetworkHosts to remove the reference to the host
		networkHostsV1 := propsv1.NewNetworkHosts()
		networkSvc := NewNetworkService(svc.provider)
		for k := range hostNetworkV1.NetworksByID {
			network, err := networkSvc.Get(k)
			if err != nil {
				log.Errorf(err.Error())
			}
			err = network.Properties.Get(NetworkProperty.HostsV1, networkHostsV1)
			if err != nil {
				log.Errorf(err.Error())
			}
			delete(networkHostsV1.ByID, host.ID)
			delete(networkHostsV1.ByName, host.Name)
			err = network.Properties.Set(NetworkProperty.HostsV1, networkHostsV1)
			if err != nil {
				log.Errorf(err.Error())
			}
			err = metadata.SaveNetwork(svc.provider, network)
			if err != nil {
				log.Errorf(err.Error())
			}
		}

		// Finally, delete metadata of host
		trydelete := mh.Delete()
		return infraErr(trydelete)
	}

	return logicErr(model.ResourceNotFoundError("host", ref))
}

// SSH returns ssh parameters to access the host referenced by ref
func (svc *HostService) SSH(ref string) (*system.SSHConfig, error) {
	host, err := svc.Get(ref)
	if err != nil {
		return nil, logicErrf(err, fmt.Sprintf("Cannot access ssh parameters of host '%s': failed to query host '%s'", ref, ref))
	}
	if host == nil {
		return nil, logicErr(fmt.Errorf("Cannot access ssh parameters of host '%s': host '%s' not found", ref, ref))
	}
	sshSvc := NewSSHService(svc.provider)
	return sshSvc.GetConfig(host.ID)
}
