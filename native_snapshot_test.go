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
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	fcvsock "github.com/firecracker-microvm/firecracker-go-sdk/vsock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// TestNativeSnapshotVsockPolicy boots a memory-only guest, continues its source
// connections, terminates that VMM, then restores into a different process.
// Run tools/test-native-snapshot.sh with the matching Firecracker and kernel.
func TestNativeSnapshotVsockPolicy(t *testing.T) {
	if os.Getenv("FC_TEST_NATIVE_SNAPSHOT") != "1" {
		t.Skip("run tools/test-native-snapshot.sh against the matching VMM")
	}
	binary, kernel, initrd := os.Getenv("FC_NATIVE_VMM"), os.Getenv("FC_NATIVE_KERNEL"), os.Getenv("FC_NATIVE_INITRD")
	for _, path := range []string{binary, kernel, initrd} {
		require.NotEmpty(t, path)
		_, err := os.Stat(path)
		require.NoError(t, err)
	}
	for _, pci := range []bool{false, true} {
		transport := "mmio"
		if pci {
			transport = "pci"
		}
		for _, retain := range []bool{false, true} {
			policy := "legacy"
			if retain {
				policy = "retain"
			}
			t.Run(transport+"/"+policy, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
				t.Cleanup(cancel)
				dir, err := os.MkdirTemp("", "fc-native-")
				require.NoError(t, err)
				t.Cleanup(func() { _ = os.RemoveAll(dir) })
				launch := func(cfg Config, opts ...Opt) *Machine {
					cfg.VMID = "native-snapshot"
					logFile, err := os.Create(filepath.Join(dir, filepath.Base(cfg.SocketPath)+".log"))
					require.NoError(t, err)
					t.Cleanup(func() {
						if t.Failed() {
							contents, _ := os.ReadFile(logFile.Name())
							t.Logf("%s:\n%s", filepath.Base(logFile.Name()), contents)
						}
						_ = logFile.Close()
					})
					logger := logrus.New()
					logger.Out = logFile
					cmd := configureBuilder(VMCommandBuilder{}.WithBin(binary).WithStdout(logFile).WithStderr(logFile), cfg).Build(ctx)
					opts = append(opts, WithProcessRunner(cmd), WithLogger(logrus.NewEntry(logger)))
					m, err := NewMachine(ctx, cfg, opts...)
					require.NoError(t, err)
					t.Cleanup(func() { require.NoError(t, m.StopVMM()) })
					require.NoError(t, m.Start(ctx))
					return m
				}
				sourceVsock := filepath.Join(dir, "source.vsock")
				source := launch(Config{
					SocketPath: filepath.Join(dir, "source.api"), KernelImagePath: kernel, InitrdPath: initrd,
					KernelArgs: "console=ttyS0 panic=1 reboot=k init=/init", EnablePCI: pci,
					MachineCfg:   models.MachineConfiguration{VcpuCount: Int64(1), MemSizeMib: Int64(128)},
					VsockDevices: []VsockDevice{{ID: "native", CID: 3, Path: sourceVsock}},
				})
				connection, err := fcvsock.DialContext(ctx, sourceVsock, 1234)
				require.NoError(t, err)
				defer connection.Close()
				first, err := nativeSnapshotRequest(connection, "set", 41)
				require.NoError(t, err)
				require.Equal(t, nativeSnapshotReply{Counter: 41, Connection: 1, Active: 1}, first)
				memPath, statePath := filepath.Join(dir, "memory"), filepath.Join(dir, "state")
				require.NoError(t, source.PauseVM(ctx))
				if retain {
					// A failed attempt must not close the source connection either.
					require.Error(t, source.CreateSnapshot(ctx, memPath, filepath.Join(dir, "absent", "state"), WithRetainedSourceVsock(true)))
					require.NoError(t, source.ResumeVM(ctx))
					stillRunning, err := nativeSnapshotRequest(connection, "get", 0)
					require.NoError(t, err)
					require.Equal(t, first, stillRunning)
					require.NoError(t, source.PauseVM(ctx))
					require.NoError(t, source.CreateSnapshot(ctx, memPath, statePath, WithRetainedSourceVsock(true)))
				} else {
					// Exercise the unchanged default by omitting both new options.
					require.NoError(t, source.CreateSnapshot(ctx, memPath, statePath))
				}
				memoryHash, stateHash := nativeSnapshotHash(t, memPath), nativeSnapshotHash(t, statePath)
				require.NoError(t, source.ResumeVM(ctx))
				continued, err := nativeSnapshotRequest(connection, "inc", 0)
				if retain {
					require.NoError(t, err)
					require.Equal(t, nativeSnapshotReply{Counter: 42, Connection: 1, Active: 1}, continued)
				} else {
					require.Error(t, err, "the legacy snapshot policy resets source connections")
				}
				oldPID, err := source.PID()
				require.NoError(t, err)
				require.NoError(t, source.StopVMM())
				waitCtx, waitCancel := context.WithTimeout(ctx, 5*time.Second)
				_ = source.Wait(waitCtx)
				waitCancel()
				_, err = os.Stat(filepath.Join("/proc", strconv.Itoa(oldPID)))
				require.True(t, os.IsNotExist(err), "source VMM must have exited before restore")
				restoredVsock := filepath.Join(dir, "restored.vsock")
				loadOpts := []WithSnapshotOpt{func(cfg *SnapshotConfig) {
					cfg.VsockOverride = &models.VsockOverride{UdsPath: String(restoredVsock)}
				}}
				if retain {
					loadOpts = append(loadOpts, WithSnapshotVsockReset(true))
				}
				restored := launch(Config{SocketPath: filepath.Join(dir, "restored.api"), EnablePCI: pci}, WithSnapshot(memPath, statePath, loadOpts...))
				newPID, err := restored.PID()
				require.NoError(t, err)
				require.NotEqual(t, oldPID, newPID)
				require.NoError(t, restored.ResumeVM(ctx))
				reconnected, err := fcvsock.DialContext(ctx, restoredVsock, 1234)
				require.NoError(t, err)
				defer reconnected.Close()
				var reply nativeSnapshotReply
				require.Eventually(t, func() bool {
					reply, err = nativeSnapshotRequest(reconnected, "get", 0)
					return err == nil && reply.Active == 1
				}, 5*time.Second, 10*time.Millisecond, "stale captured connection must be reset")
				require.Equal(t, nativeSnapshotReply{Counter: 41, Connection: 2, Active: 1}, reply)
				reply, err = nativeSnapshotRequest(reconnected, "inc", 0)
				require.NoError(t, err)
				require.Equal(t, uint64(42), reply.Counter)
				require.Equal(t, memoryHash, nativeSnapshotHash(t, memPath), "restored writes must not alter snapshot memory")
				require.Equal(t, stateHash, nativeSnapshotHash(t, statePath))
				t.Logf("source_pid=%d restored_pid=%d counter=%d connection=%d active=%d", oldPID, newPID, reply.Counter, reply.Connection, reply.Active)
			})
		}
	}
}

type nativeSnapshotReply struct{ Counter, Connection, Active uint64 }

func nativeSnapshotRequest(conn net.Conn, op string, value uint64) (nativeSnapshotReply, error) {
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return nativeSnapshotReply{}, err
	}
	if err := json.NewEncoder(conn).Encode(struct {
		Op    string
		Value uint64
	}{op, value}); err != nil {
		return nativeSnapshotReply{}, err
	}
	var reply nativeSnapshotReply
	err := json.NewDecoder(conn).Decode(&reply)
	return reply, err
}

func nativeSnapshotHash(t *testing.T, path string) string {
	t.Helper()
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	require.NoError(t, err)
	return string(hash.Sum(nil))
}
