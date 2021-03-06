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
	"github.com/CS-SI/SafeScale/providers"
	"github.com/CS-SI/SafeScale/providers/api"
	"github.com/CS-SI/SafeScale/providers/metadata"
	"github.com/CS-SI/SafeScale/providers/model"
	"github.com/CS-SI/SafeScale/providers/model/enums/VolumeSpeed"
	"github.com/CS-SI/SafeScale/providers/openstack"
)

// providerNetwork name of ovh external network
const providerNetwork string = "Ext-Net"

type gpuCfg struct {
	GPUNumber int
	GPUType   string
}

var gpuMap = map[string]gpuCfg{
	"g2-15": gpuCfg{
		GPUNumber: 1,
		GPUType:   "NVIDIA 1070",
	},
	"g2-30": gpuCfg{
		GPUNumber: 1,
		GPUType:   "NVIDIA 1070",
	},
	"g3-120": gpuCfg{
		GPUNumber: 3,
		GPUType:   "NVIDIA 1080 TI",
	},
	"g3-30": gpuCfg{
		GPUNumber: 1,
		GPUType:   "NVIDIA 1080 TI",
	},
}

/*AuthOptions fields are the union of those recognized by each identity implementation and
provider.
*/
type AuthOptions struct {
	// // Endpoint ovh end point (ovh-eu, ovh-ca ...)
	// Endpoint string
	// //Application or Project Name
	// ApplicationName string
	// Application Key or project ID
	ApplicationKey string
	// //Consumer key
	// ConsumerKey string
	// Openstack identifier
	OpenstackID string
	// OpenStack password
	OpenstackPassword string
	// Name of the data center (GRA3, BHS3 ...)
	Region string
	// Project Name
	ProjectName string
}

// func parseOpenRC(openrc string) (*openstack.AuthOptions, error) {
// 	tokens := strings.Split(openrc, "export")
// }

// AuthenticatedClient returns an authenticated client
func AuthenticatedClient(opts AuthOptions, cfg openstack.CfgOptions) (*Client, error) {
	client := &Client{}
	osclt, err := openstack.AuthenticatedClient(
		openstack.AuthOptions{
			IdentityEndpoint: "https://auth.cloud.ovh.net/v2.0",
			//UserID:           opts.OpenstackID,
			Username:    opts.OpenstackID,
			Password:    opts.OpenstackPassword,
			TenantID:    opts.ApplicationKey,
			TenantName:  opts.ProjectName,
			Region:      opts.Region,
			AllowReauth: true,
		},
		openstack.CfgOptions{
			ProviderNetwork:           providerNetwork,
			UseFloatingIP:             false,
			UseLayer3Networking:       false,
			AutoHostNetworkInterfaces: false,
			DNSList:                   []string{"213.186.33.99", "1.1.1.1"},
			VolumeSpeeds: map[string]VolumeSpeed.Enum{
				"classic":    VolumeSpeed.COLD,
				"high-speed": VolumeSpeed.HDD,
			},
			MetadataBucket: metadata.BuildMetadataBucketName(opts.ApplicationKey),
			DefaultImage:   cfg.DefaultImage,
		},
	)

	if err != nil {
		return nil, err
	}
	client.osclt = osclt

	return client, nil

}

// Client is the implementation of the ovh driver regarding to the api.ClientAPI
// This client used ovh api and opensatck ovh api to maximize code reuse
type Client struct {
	osclt *openstack.Client
	opts  AuthOptions
}

// Build build a new Client from configuration parameter
func (client *Client) Build(params map[string]interface{}) (api.ClientAPI, error) {
	// tenantName, _ := params["name"].(string)

	identity, _ := params["identity"].(map[string]interface{})
	compute, _ := params["compute"].(map[string]interface{})
	// network, _ := params["network"].(map[string]interface{})

	applicationKey, _ := identity["ApplicationKey"].(string)
	openstackID, _ := identity["OpenstackID"].(string)
	openstackPassword, _ := identity["OpenstackPassword"].(string)

	region, _ := compute["Region"].(string)
	projectName, _ := compute["ProjectName"].(string)
	defaultImage, _ := compute["DefaultImage"].(string)

	return AuthenticatedClient(
		AuthOptions{
			ApplicationKey:    applicationKey,
			OpenstackID:       openstackID,
			OpenstackPassword: openstackPassword,
			Region:            region,
			ProjectName:       projectName,
		},
		openstack.CfgOptions{
			DefaultImage: defaultImage,
		},
	)
}

// GetCfgOpts return configuration parameters
func (client *Client) GetCfgOpts() (model.Config, error) {
	return client.osclt.GetCfgOpts()
}

// GetAuthOpts returns the auth options
func (client *Client) GetAuthOpts() (model.Config, error) {
	return client.osclt.GetAuthOpts()
}

func init() {
	providers.Register("ovh", &Client{})
}
