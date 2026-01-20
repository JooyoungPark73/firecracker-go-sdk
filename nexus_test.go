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
	shmemPath := "/dev/shm/test_region"
	var sizeMib int64 = 16

	device := NewNexusDevice(id, shmemPath, sizeMib)
	nexus := device.Build()

	if nexus.ID == nil || *nexus.ID != id {
		t.Errorf("Expected ID %s, got %v", id, nexus.ID)
	}

	if nexus.ShmemPath == nil || *nexus.ShmemPath != shmemPath {
		t.Errorf("Expected ShmemPath %s, got %v", shmemPath, nexus.ShmemPath)
	}

	if nexus.SizeMib == nil || *nexus.SizeMib != sizeMib {
		t.Errorf("Expected SizeMib %d, got %v", sizeMib, nexus.SizeMib)
	}
}

func TestNexusDeviceOptions(t *testing.T) {
	id := "nexus1"
	shmemPath := "/dev/shm/test_region2"
	var sizeMib int64 = 32

	device := NewNexusDevice("initial_id", "/dev/shm/initial", 8,
		WithNexusID(id),
		WithShmemPath(shmemPath),
		WithSizeMib(sizeMib),
	)

	nexus := device.Build()

	if nexus.ID == nil || *nexus.ID != id {
		t.Errorf("Expected ID %s, got %v", id, nexus.ID)
	}

	if nexus.ShmemPath == nil || *nexus.ShmemPath != shmemPath {
		t.Errorf("Expected ShmemPath %s, got %v", shmemPath, nexus.ShmemPath)
	}

	if nexus.SizeMib == nil || *nexus.SizeMib != sizeMib {
		t.Errorf("Expected SizeMib %d, got %v", sizeMib, nexus.SizeMib)
	}
}

func TestNexusDeviceValidation(t *testing.T) {
	testCases := []struct {
		name      string
		id        string
		shmemPath string
		sizeMib   int64
		valid     bool
	}{
		{
			name:      "Valid nexus device",
			id:        "nexus0",
			shmemPath: "/dev/shm/nexus_region",
			sizeMib:   16,
			valid:     true,
		},
		{
			name:      "Valid with larger size",
			id:        "nexus1",
			shmemPath: "/dev/shm/large_region",
			sizeMib:   1024,
			valid:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			device := NewNexusDevice(tc.id, tc.shmemPath, tc.sizeMib)
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
	shmemPath := "/dev/shm/test"
	var sizeMib int64 = 64

	device := NewNexusDevice(id, shmemPath, sizeMib)
	nexus1 := device.Build()
	nexus2 := device.Build()

	// Verify both builds return the same values
	if *nexus1.ID != *nexus2.ID {
		t.Error("Multiple Build() calls should return consistent ID")
	}

	if *nexus1.ShmemPath != *nexus2.ShmemPath {
		t.Error("Multiple Build() calls should return consistent ShmemPath")
	}

	if *nexus1.SizeMib != *nexus2.SizeMib {
		t.Error("Multiple Build() calls should return consistent SizeMib")
	}
}

func TestNexusWithFunctionalOptions(t *testing.T) {
	// Test that functional options work correctly
	opts := []NexusOpt{
		WithNexusID("custom_id"),
		WithShmemPath("/custom/path"),
		WithSizeMib(128),
	}

	device := NewNexusDevice("initial", "/initial", 1, opts...)
	nexus := device.Build()

	if *nexus.ID != "custom_id" {
		t.Errorf("Expected ID 'custom_id', got '%s'", *nexus.ID)
	}

	if *nexus.ShmemPath != "/custom/path" {
		t.Errorf("Expected ShmemPath '/custom/path', got '%s'", *nexus.ShmemPath)
	}

	if *nexus.SizeMib != 128 {
		t.Errorf("Expected SizeMib 128, got %d", *nexus.SizeMib)
	}
}
