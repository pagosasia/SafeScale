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

package ovh

import (
	"strings"
	"time"

	filters "github.com/CS-SI/SafeScale/providers/filters/templates"
	"github.com/CS-SI/SafeScale/providers/model"
	"github.com/CS-SI/SafeScale/providers/model/enums/HostState"
)

// ListAvailabilityZones ...
func (client *Client) ListAvailabilityZones(all bool) (map[string]bool, error) {
	return client.osclt.ListAvailabilityZones(all)
}

// ListImages lists available OS images
func (client *Client) ListImages(all bool) ([]model.Image, error) {
	return client.osclt.ListImages(all)
}

// GetImage returns the Image referenced by id
func (client *Client) GetImage(id string) (*model.Image, error) {
	return client.osclt.GetImage(id)
}

// GetTemplate overload OpenStack GetTemplate method to add GPU configuration
func (client *Client) GetTemplate(id string) (*model.HostTemplate, error) {
	tpl, err := client.osclt.GetTemplate(id)
	if tpl != nil {
		addGPUCfg(tpl)
	}
	return tpl, err
}

func addGPUCfg(tpl *model.HostTemplate) {
	if cfg, ok := gpuMap[tpl.Name]; ok {
		tpl.GPUNumber = cfg.GPUNumber
		tpl.GPUType = cfg.GPUType
	}
}

// ListTemplates overload OpenStack ListTemplate method to filter wind and flex instance and add GPU configuration
func (client *Client) ListTemplates(all bool) ([]model.HostTemplate, error) {
	allTemplates, err := client.osclt.ListTemplates(all)
	if err != nil {
		return nil, err
	}
	if all {
		return allTemplates, nil
	}

	filter := filters.NewFilter(isWindowsTemplate).Not().And(filters.NewFilter(isFlexTemplate).Not())
	return filters.FilterTemplates(allTemplates, filter), nil
}

func isWindowsTemplate(t model.HostTemplate) bool {
	return strings.HasPrefix(strings.ToLower(t.Name), "win-")
}
func isFlexTemplate(t model.HostTemplate) bool {
	return strings.HasSuffix(strings.ToLower(t.Name), "flex")
}

// CreateKeyPair creates and import a key pair
func (client *Client) CreateKeyPair(name string) (*model.KeyPair, error) {
	return client.osclt.CreateKeyPair(name)
}

// GetKeyPair returns the key pair identified by id
func (client *Client) GetKeyPair(id string) (*model.KeyPair, error) {
	return client.osclt.GetKeyPair(id)
}

// ListKeyPairs lists available key pairs
func (client *Client) ListKeyPairs() ([]model.KeyPair, error) {
	return client.osclt.ListKeyPairs()
}

// DeleteKeyPair deletes the key pair identified by id
func (client *Client) DeleteKeyPair(id string) error {
	return client.osclt.DeleteKeyPair(id)
}

// CreateHost creates an host satisfying request
func (client *Client) CreateHost(request model.HostRequest) (*model.Host, error) {
	return client.osclt.CreateHost(request)
}

// WaitHostReady waits an host achieve ready state
func (client *Client) WaitHostReady(hostID string, timeout time.Duration) (*model.Host, error) {
	return client.osclt.WaitHostReady(hostID, timeout)
}

// GetHost returns the host identified by id
func (client *Client) GetHost(hostParam interface{}) (*model.Host, error) {
	return client.osclt.GetHost(hostParam)
}

// GetHostByName ...
func (client *Client) GetHostByName(name string) (*model.Host, error) {
	return client.osclt.GetHostByName(name)
}

// GetHostState returns the host identified by id
func (client *Client) GetHostState(hostParam interface{}) (HostState.Enum, error) {
	return client.osclt.GetHostState(hostParam)
}

// ListHosts lists all hosts
func (client *Client) ListHosts() ([]*model.Host, error) {
	return client.osclt.ListHosts()
}

// DeleteHost deletes the host identified by id
func (client *Client) DeleteHost(id string) error {
	return client.osclt.DeleteHost(id)
}

// StopHost stops the host identified by id
func (client *Client) StopHost(id string) error {
	return client.osclt.StopHost(id)
}

// RebootHost ...
func (client *Client) RebootHost(id string) error {
	return client.osclt.RebootHost(id)
}

// StartHost starts the host identified by id
func (client *Client) StartHost(id string) error {
	return client.osclt.StartHost(id)
}

// // GetSSHConfig creates SSHConfig to connect an host
// // param can be type string or *model.Host; any other type will panic
// func (client *Client) GetSSHConfig(hostParam interface{}) (*system.SSHConfig, error) {
// 	return client.osclt.GetSSHConfig(hostParam)
// }
