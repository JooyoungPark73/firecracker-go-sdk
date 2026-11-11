# Source continuation and restored vsock connections

The matching `cri-native-snapshot-restore` Firecracker fork at commit
`ce86b117d59ca93b1c04b791a251b445ace76703` exposes two snapshot options.
Use them together when the source must continue with its established
vsock connections:

```go
err := source.PauseVM(ctx)
// Handle err before continuing.
err = source.CreateSnapshot(ctx, memoryPath, statePath,
    firecracker.WithRetainedSourceVsock(true))
// Handle err and resume the source as part of checkpoint cleanup.
err = source.ResumeVM(ctx)

restored, err := firecracker.NewMachine(ctx, restoredConfig,
    firecracker.WithSnapshot(memoryPath, statePath,
        firecracker.WithSnapshotVsockReset(true)))
```

The source keeps its existing connections. On restore, listening guest sockets
survive while established connections reset and must reconnect. Backend
connections are not serialized. A snapshot manager must record and enforce this
pairing; neither option changes the snapshot state format. Existing defaults
continue to reset connections during snapshot creation.

The Swagger models and client are generated using `go generate ./` with
go-swagger v0.33.1. The SDK Swagger file must match the selected VMM Swagger
exactly.

On the designated worker, build the matching VMM and select the pinned guest
kernel before running:

```sh
FC_NATIVE_VMM=/absolute/path/to/firecracker \
FC_NATIVE_KERNEL=/absolute/path/to/default-vmlinux.bin \
    tools/test-native-snapshot.sh
```

The script builds a static PID 1 fixture and a temporary initramfs, then runs
real VMMs with MMIO and PCI devices. It verifies the default reset behavior,
source connection survival after failed and successful retained snapshots,
source VMM termination before restore, restored memory counters, stale guest
connection cleanup, a working listener/reconnection and immutable snapshot
files. Its guest device nodes come from devtmpfs.

This is a VMM transport and memory test. Full Pod restore additionally needs
consistent independent writable disks, restored runtime metadata and ownership,
task IO/events, networking, volumes and CRI/Kubernetes integration.
