// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//	http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.
package firecracker

import (
	models "github.com/firecracker-microvm/firecracker-go-sdk/client/models"
)

// NexusDevice is a builder that will create a nexus shared memory device
// used to set up the firecracker microVM.
type NexusDevice struct {
	nexus models.Nexus
}

// NexusOpt is a functional option for configuring a nexus device.
type NexusOpt func(*models.Nexus)

// NewNexusDevice creates a new NexusDevice builder with the specified configuration.
func NewNexusDevice(id, pathOnHost string, opts ...NexusOpt) NexusDevice {
	n := models.Nexus{
		ID:         &id,
		PathOnHost: &pathOnHost,
	}

	for _, opt := range opts {
		opt(&n)
	}

	return NexusDevice{nexus: n}
}

// Build will return the configured nexus device.
func (n NexusDevice) Build() models.Nexus {
	return n.nexus
}

// WithNexusID sets the ID of the nexus device.
func WithNexusID(id string) NexusOpt {
	return func(n *models.Nexus) {
		n.ID = String(id)
	}
}

// WithPathOnHost sets the host path of the nexus device backing file.
func WithPathOnHost(path string) NexusOpt {
	return func(n *models.Nexus) {
		n.PathOnHost = String(path)
	}
}
