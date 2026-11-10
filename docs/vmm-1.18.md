# Firecracker 1.18 compatibility

This branch retains `feat/update-firecracker-1.14` at
`5368ef7c727702672d4942a8f8b344134f41ef82` and updates its generated API to
upstream Firecracker **v1.18.0**, commit
`eaf83bb718a559d324613d566eaf776b31a62340` (annotated tag object
`07a7c2e20a8f4f661702551d02fc3eca679842a5`).

`client/swagger.yaml` is copied unchanged from
`src/firecracker/swagger/firecracker.yaml` at that commit. Its SHA-256 is
`3693a55005be2b9c81bf14e9908bd894e361adedff4abcf8c1ac24a8c40adb0c`.
Models, operations and mocks are generated with Go Swagger **v0.33.1**:

```sh
go generate ./
make unit-tests
FC_TEST_RELEASE_API=1 DISABLE_ROOT_TESTS=1 go test -short -timeout 90s ./...
sudo env PATH="$PATH" GOMODCACHE="$(go env GOMODCACHE)" \
  FC_TEST_BIN="$PWD/testdata/firecracker" FC_TEST_DATA_PATH="$PWD/testdata" \
  go test -v -run '^TestWait$' -timeout 90s
```

These commands were run on `amd218.utah.cloudlab.us` (Ubuntu 24.04.5,
Linux 6.8.0-147-generic, AMD EPYC 7302P, Go **1.26.11**). Non-root unit tests,
the real VMM API test, and all four KVM-backed TestWait lifecycle cases pass.
Regenerating the client produces identical file hashes. Test binaries are
`testdata/firecracker` and `testdata/jailer`, downloaded from the fixed upstream
v1.18.0 release and verified by SHA-256. `tools/prepare-testdata.sh` pins the
guest kernel, rootfs and release archives; no S3 listing or mutable branch is
used. Fixture generation currently covers x86_64 only; ARM worker validation
has not been performed.

The previous helper additions for pmem, serial, memory hotplug and balloon
hinting remain available. The v1.18.0 schema adds device options and APIs,
including vsock overrides, pmem rate limiting and Nitro Sandbox configuration.
Balloon hinting start/stop now return HTTP 204 and memory-hotplug update size is
a required pointer. Snapshot load errors retain their underlying error chain.
`TestReleaseAPI` verifies the real VMM version, machine configuration,
free-page reporting/hinting configuration and typed error responses. It does
not claim to validate guest reclamation or snapshot consistency.

`make all-tests` additionally prepares an Ubuntu 24.04 ext4 SSH fixture, with
locally generated test-only credentials, and CNI **v1.9.1** and
`tc-redirect-tap` **v0.0.0-20250516183331-34bf829e9a5c**. Generated SSH credentials
and large disk fixtures are ignored by Git; an external `FC_TEST_DATA_PATH` is
recommended for worker testing.

The complete privileged suite now passes on the worker, including native jailer,
balloon updates, CNI/networking, snapshot creation, file memory-backend restore,
and restore checked through SSH. The existing `TestLogAndMetrics` remains skipped
by its own test code. These tests use the verified v1.18.0 release binaries and the
pinned fixture inputs. The container-based test runner uses `--network=none` to
isolate test networking and `--cgroupns=host` so the native jailer can create cgroup
v2 cpusets; its test cgroup must be absent before the run and removed afterward.

The snapshot wrapper normalizes a nil network-override list to an empty JSON
sequence without changing the caller's model. A nil list emitted by the generated
model was rejected by the real VMM. `TestReleaseAPI` now covers that regression as
well. The kernel fixture's boot arguments no longer disable APIC, which prevented
virtio activation with the 6.18 kernel.

The runtime repository records the full privileged runner command and worker
logs. Passing this SDK suite does not establish runtime-level snapshot storage
consistency or complete the firecracker-containerd modernization milestone.
