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

package api

import (
	"github.com/CS-SI/SafeScale/providers/model"
	"github.com/CS-SI/SafeScale/providers/model/enums/HostState"
)

//go:generate mockgen -destination=../mocks/mock_clientapi.go -package=mocks github.com/CS-SI/SafeScale/providers/api ClientAPI

// ClientAPI is an API defining an IaaS driver
type ClientAPI interface {
	Build(map[string]interface{}) (ClientAPI, error)

	// ListAvailabilityZones lists the usable Availability Zones
	ListAvailabilityZones(bool) (map[string]bool, error)

	// ListImages lists available OS images
	ListImages(all bool) ([]model.Image, error)
	// GetImage returns the Image referenced by id
	GetImage(id string) (*model.Image, error)

	// GetTemplate returns the Template referenced by id
	GetTemplate(id string) (*model.HostTemplate, error)
	// ListTemplates lists available host templates
	// Host templates are sorted using Dominant Resource Fairness Algorithm
	ListTemplates(all bool) ([]model.HostTemplate, error)

	// CreateKeyPair creates and import a key pair
	CreateKeyPair(name string) (*model.KeyPair, error)
	// GetKeyPair returns the key pair identified by id
	GetKeyPair(id string) (*model.KeyPair, error)
	// ListKeyPairs lists available key pairs
	ListKeyPairs() ([]model.KeyPair, error)
	// DeleteKeyPair deletes the key pair identified by id
	DeleteKeyPair(id string) error

	// CreateNetwork creates a network named name
	CreateNetwork(req model.NetworkRequest) (*model.Network, error)
	// GetNetwork returns the network identified by id
	GetNetwork(id string) (*model.Network, error)
	// GetNetworkByName returns the network identified by name)
	GetNetworkByName(name string) (*model.Network, error)
	// ListNetworks lists all networks
	ListNetworks() ([]*model.Network, error)
	// DeleteNetwork deletes the network identified by id
	DeleteNetwork(id string) error
	// CreateGateway creates a public Gateway for a private network
	CreateGateway(req model.GatewayRequest) (*model.Host, error)
	// DeleteGateway ...
	DeleteGateway(string) error

	// CreateHost creates an host that fulfils the request
	CreateHost(request model.HostRequest) (*model.Host, error)
	// GetHost returns the host identified by id or updates content of a *model.Host
	GetHost(interface{}) (*model.Host, error)
	// GetHostByName returns the host identified by name
	GetHostByName(string) (*model.Host, error)
	// GetHostState returns the current state of the host identified by id
	GetHostState(interface{}) (HostState.Enum, error)
	// ListHosts lists all hosts
	ListHosts() ([]*model.Host, error)
	// DeleteHost deletes the host identified by id
	DeleteHost(id string) error
	// StopHost stops the host identified by id
	StopHost(id string) error
	// StartHost starts the host identified by id
	StartHost(id string) error
	// GetSSHConfig creates SSHConfig from host
	//GetSSHConfig(param interface{}) (*system.SSHConfig, error)
	// Reboot host
	RebootHost(id string) error

	// CreateVolume creates a block volume
	// - name is the name of the volume
	// - size is the size of the volume in GB
	// - volumeType is the type of volume to create, if volumeType is empty the driver use a default type
	CreateVolume(request model.VolumeRequest) (*model.Volume, error)
	// GetVolume returns the volume identified by id
	GetVolume(id string) (*model.Volume, error)
	// ListVolumes list available volumes
	ListVolumes() ([]model.Volume, error)
	// DeleteVolume deletes the volume identified by id
	DeleteVolume(id string) error

	// CreateVolumeAttachment attaches a volume to an host
	//- name of the volume attachment
	//- volume to attach
	//- host on which the volume is attached
	CreateVolumeAttachment(request model.VolumeAttachmentRequest) (string, error)
	// GetVolumeAttachment returns the volume attachment identified by id
	GetVolumeAttachment(serverID, id string) (*model.VolumeAttachment, error)
	// ListVolumeAttachments lists available volume attachment
	ListVolumeAttachments(serverID string) ([]model.VolumeAttachment, error)
	// DeleteVolumeAttachment deletes the volume attachment identifed by id
	DeleteVolumeAttachment(serverID, id string) error

	// // CreateBucket creates an object container
	// CreateBucket(bucketName string) error
	// // DeleteBucket deletes an object container
	// DeleteBucket(bucketName string) error
	// // ListBuckets list object containers
	// ListBuckets() ([]string, error)
	// // GetBucket returns info about the Bucket
	// GetBucket(name string) (*model.Bucket, error)
	// // PutObject put an object into a Bucket
	// PutObject(bucketName string, obj model.Object) error
	// // UpdateObjectMetadata update an object into  object container
	// UpdateObjectMetadata(bucketName string, obj model.Object) error
	// // GetObject get object content from a Bucket
	// GetObject(bucketName string, objectName string, ranges []model.Range) (*model.Object, error)
	// // GetObjectMetadata get object metadata from a Bucket
	// GetObjectMetadata(bucketName string, objectName string) (*model.Object, error)
	// // ListObjects list objects of a container
	// ListObjects(buckeName string, filter model.ObjectFilter) ([]string, error)
	// // CopyObject copies an object
	// CopyObject(bucketNameSrc, objectSrc, objectDst string) error
	// // DeleteObject delete an object from a container
	// DeleteObject(bucketName, objectName string) error

	// GetAuthOpts returns authentification options as a Config
	GetAuthOpts() (model.Config, error)
	// GetCfgOpts returns configuration options as a Config
	GetCfgOpts() (model.Config, error)
}
