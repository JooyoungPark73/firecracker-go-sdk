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
	"testing"
)

func TestNewNexusDevice(t *testing.T) {
	id := "nexus0"
	pathOnHost := "/dev/shm/test_region"

	device := NewNexusDevice(id, pathOnHost)
	nexus := device.Build()

	if nexus.ID == nil || *nexus.ID != id {
		t.Errorf("Expected ID %s, got %v", id, nexus.ID)
	}

	if nexus.PathOnHost == nil || *nexus.PathOnHost != pathOnHost {
		t.Errorf("Expected PathOnHost %s, got %v", pathOnHost, nexus.PathOnHost)
	}
}

func TestNexusDeviceOptions(t *testing.T) {
	id := "nexus1"
	pathOnHost := "/dev/shm/test_region2"

	device := NewNexusDevice("initial_id", "/dev/shm/initial",
		WithNexusID(id),
		WithPathOnHost(pathOnHost),
	)

	nexus := device.Build()

	if nexus.ID == nil || *nexus.ID != id {
		t.Errorf("Expected ID %s, got %v", id, nexus.ID)
	}

	if nexus.PathOnHost == nil || *nexus.PathOnHost != pathOnHost {
		t.Errorf("Expected PathOnHost %s, got %v", pathOnHost, nexus.PathOnHost)
	}
}

func TestNexusDeviceValidation(t *testing.T) {
	testCases := []struct {
		name       string
		id         string
		pathOnHost string
		valid      bool
	}{
		{
			name:       "Valid nexus device",
			id:         "nexus0",
			pathOnHost: "/dev/shm/nexus_region",
			valid:      true,
		},
		{
			name:       "Valid with different path",
			id:         "nexus1",
			pathOnHost: "/dev/shm/large_region",
			valid:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			device := NewNexusDevice(tc.id, tc.pathOnHost)
			nexus := device.Build()

			err := nexus.Validate(nil)
			if tc.valid && err != nil {
				t.Errorf("Expected valid nexus device, got error: %v", err)
			}
			if !tc.valid && err == nil {
				t.Errorf("Expected invalid nexus device, got no error")
			}
		})
	}
}

func TestNexusDeviceBuild(t *testing.T) {
	id := "test_nexus"
	pathOnHost := "/dev/shm/test"

	device := NewNexusDevice(id, pathOnHost)
	nexus1 := device.Build()
	nexus2 := device.Build()

	// Verify both builds return the same values
	if *nexus1.ID != *nexus2.ID {
		t.Error("Multiple Build() calls should return consistent ID")
	}

	if *nexus1.PathOnHost != *nexus2.PathOnHost {
		t.Error("Multiple Build() calls should return consistent PathOnHost")
	}
}

func TestNexusWithFunctionalOptions(t *testing.T) {
	// Test that functional options work correctly
	opts := []NexusOpt{
		WithNexusID("custom_id"),
		WithPathOnHost("/custom/path"),
	}

	device := NewNexusDevice("initial", "/initial", opts...)
	nexus := device.Build()

	if *nexus.ID != "custom_id" {
		t.Errorf("Expected ID 'custom_id', got '%s'", *nexus.ID)
	}

	if *nexus.PathOnHost != "/custom/path" {
		t.Errorf("Expected PathOnHost '/custom/path', got '%s'", *nexus.PathOnHost)
	}
}
