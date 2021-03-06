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
	"github.com/CS-SI/SafeScale/providers/model"
)

// CreateVolume creates a block volume
// - name is the name of the volume
// - size is the size of the volume in GB
// - volumeType is the type of volume to create, if volumeType is empty the driver use a default type
func (client *Client) CreateVolume(request model.VolumeRequest) (*model.Volume, error) {
	return client.osclt.CreateVolume(request)
}

// GetVolume returns the volume identified by id
func (client *Client) GetVolume(id string) (*model.Volume, error) {
	return client.osclt.GetVolume(id)
}

// ListVolumes lists volume IDs
func (client *Client) ListVolumes() ([]model.Volume, error) {
	return client.osclt.ListVolumes()
}

// DeleteVolume deletes the volume identified by id
func (client *Client) DeleteVolume(id string) error {
	return client.osclt.DeleteVolume(id)
}

// CreateVolumeAttachment attaches a volume to an host
// - 'name' of the volume attachment
// - 'volume' to attach
// - 'host' on which the volume is attached
func (client *Client) CreateVolumeAttachment(request model.VolumeAttachmentRequest) (string, error) {
	return client.osclt.CreateVolumeAttachment(request)
}

// DeleteVolumeAttachment ...
func (client *Client) DeleteVolumeAttachment(serverID, vaID string) error {
	return client.osclt.DeleteVolumeAttachment(serverID, vaID)
}

// GetVolumeAttachment returns the volume attachment identified by id
func (client *Client) GetVolumeAttachment(serverID, id string) (*model.VolumeAttachment, error) {
	return client.osclt.GetVolumeAttachment(serverID, id)
}

// ListVolumeAttachments lists available volume attachment
func (client *Client) ListVolumeAttachments(serverID string) ([]model.VolumeAttachment, error) {
	return client.osclt.ListVolumeAttachments(serverID)
}
