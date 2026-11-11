#!/usr/bin/env bash
# Run only on the designated build/test worker with its pinned VMM and kernel.
set -euo pipefail
cd "$(dirname "$0")/.."
: "${FC_NATIVE_VMM:?set FC_NATIVE_VMM to the matching built Firecracker}"
: "${FC_NATIVE_KERNEL:?set FC_NATIVE_KERNEL to the pinned guest kernel}"
FC_NATIVE_VMM=$(realpath "$FC_NATIVE_VMM")
FC_NATIVE_KERNEL=$(realpath "$FC_NATIVE_KERNEL")
fixture_dir=$(mktemp -d /tmp/fc-native-fixture.XXXXXX)
trap 'rm -rf "$fixture_dir"' EXIT
CGO_ENABLED=0 go build -mod=readonly -trimpath -o "$fixture_dir/init" ./tools/native-snapshot-guest
python3 - "$fixture_dir/init" "$fixture_dir/initrd.cpio" <<'PY'
import pathlib
import stat
import sys

with open(sys.argv[2], "wb") as archive:
    def entry(name, mode, data=b"", major=0, minor=0):
        filename = name.encode() + b"\0"
        fields = (1, mode, 0, 0, 1, 0, len(data), 0, 0, major, minor, len(filename), 0)
        header = b"070701" + b"".join(f"{value:08x}".encode() for value in fields)
        archive.write(header + filename)
        archive.write(b"\0" * (-len(header + filename) % 4))
        archive.write(data)
        archive.write(b"\0" * (-len(data) % 4))

    for name in ("dev", "proc", "sys"):
        entry(name, stat.S_IFDIR | 0o755)
    entry("dev/console", stat.S_IFCHR | 0o600, major=5, minor=1)
    entry("init", stat.S_IFREG | 0o755, pathlib.Path(sys.argv[1]).read_bytes())
    entry("TRAILER!!!", 0)
PY
go test -mod=readonly -c -o "$fixture_dir/native-snapshot.test" .
sudo -n env FC_TEST_NATIVE_SNAPSHOT=1 FC_NATIVE_VMM="$FC_NATIVE_VMM" \
    FC_NATIVE_KERNEL="$FC_NATIVE_KERNEL" FC_NATIVE_INITRD="$fixture_dir/initrd.cpio" \
    "$fixture_dir/native-snapshot.test" -test.v -test.count=1 -test.timeout=420s \
    -test.run='^TestNativeSnapshotVsockPolicy$'
