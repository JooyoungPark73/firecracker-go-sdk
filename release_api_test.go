// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package firecracker

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/operations"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// Run explicitly against the pinned real VMM; no VM or KVM access is required
// to check the pre-boot device API and its error responses.
func TestReleaseAPI(t *testing.T) {
	if os.Getenv("FC_TEST_RELEASE_API") != "1" {
		t.Skip("set FC_TEST_RELEASE_API=1 to exercise the pinned Firecracker binary")
	}
	binary, err := filepath.Abs(envOrDefault(firecrackerBinaryOverrideEnv, filepath.Join(testDataPath, "firecracker")))
	require.NoError(t, err)
	version, err := exec.Command(binary, "--version").CombinedOutput()
	require.NoError(t, err)
	require.Contains(t, string(version), "Firecracker v"+Version)

	socket := filepath.Join(t.TempDir(), "api.sock")
	var output bytes.Buffer
	cmd := exec.Command(binary, "--api-sock", socket)
	cmd.Stdout, cmd.Stderr = &output, &output
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if t.Failed() {
			t.Log(output.String())
		}
	})
	require.Eventually(t, func() bool { _, err := os.Stat(socket); return err == nil }, 5*time.Second, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	api := NewClient(socket, logrus.NewEntry(logrus.New()), false)
	actual, err := api.GetFirecrackerVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, Version, StringValue(actual.Payload.FirecrackerVersion))
	_, err = api.PutMachineConfiguration(ctx, &models.MachineConfiguration{VcpuCount: Int64(1), MemSizeMib: Int64(128)})
	require.NoError(t, err)
	_, err = api.PutBalloon(ctx, &models.Balloon{
		AmountMib: Int64(0), DeflateOnOom: Bool(true),
		FreePageReporting: true, FreePageHinting: true,
	})
	require.NoError(t, err)
	balloon, err := api.DescribeBalloonConfig(ctx)
	require.NoError(t, err)
	require.True(t, balloon.Payload.FreePageReporting)
	require.True(t, balloon.Payload.FreePageHinting)
	_, err = api.PutMachineConfiguration(ctx, &models.MachineConfiguration{VcpuCount: Int64(0), MemSizeMib: Int64(128)})
	var badRequest *operations.PutMachineConfigurationBadRequest
	require.ErrorAs(t, err, &badRequest)
}
