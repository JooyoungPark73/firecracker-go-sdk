# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
# 	http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

# Set this to pass additional commandline flags to the go compiler, e.g. "make test EXTRAGOARGS=-v"
CARGO_CACHE_VOLUME_NAME?=firecracker-go-sdk--cargocache
DISABLE_ROOT_TESTS?=1
DOCKER_IMAGE_TAG?=latest
EXTRAGOARGS:=
FIRECRACKER_DIR=build/firecracker
ARCH=$(shell uname -m)
FIRECRACKER_TARGET?=$(ARCH)-unknown-linux-musl

FC_TEST_DATA_PATH?=testdata
FC_TEST_BIN_PATH:=$(FC_TEST_DATA_PATH)/bin
FIRECRACKER_BIN=$(FC_TEST_DATA_PATH)/firecracker-main
JAILER_BIN=$(FC_TEST_DATA_PATH)/jailer-main

UID = $(shell id -u)
GID = $(shell id -g)

# Version has to be in format of vx.x.x
firecracker_version=v1.14
CI_VERSION=$(basename $(firecracker_version))
KERNEL_VERSION=6.1
ubuntu_version=24.04

# URL to download CI artifacts
### Kernel
latest_kernel_key=$(wget "http://spec.ccfc.min.s3.amazonaws.com/?prefix=firecracker-ci/$(CI_VERSION)/$(ARCH)/vmlinux-$(KERNEL_VERSION)&list-type=2" -O - 2>/dev/null \
	| grep "(?<=<Key>)(firecracker-ci/$(CI_VERSION)/$(ARCH)/vmlinux-$(KERNEL_VERSION)\.[0-9]{3})(?=</Key>)" -o -P)

kernel_url=https://s3.amazonaws.com/spec.ccfc.min/${latest_kernel_key}

### Ubuntu
latest_ubuntu_key=$(curl "http://spec.ccfc.min.s3.amazonaws.com/?prefix=firecracker-ci/$(CI_VERSION)/$(ARCH)/ubuntu-$(ubuntu_version)&list-type=2" \
    | grep -oP "(?<=<Key>)(firecracker-ci/$CI_VERSION/$ARCH/ubuntu-[0-9]+\.[0-9]+\.squashfs)(?=</Key>)" \
    | sort -V | tail -1)
ubuntu_version=$(basename $latest_ubuntu_key .squashfs | grep -oE '[0-9]+\.[0-9]+')
ubuntu_rootfs_url=https://s3.amazonaws.com/spec.ccfc.min/${latest_ubuntu_key}

# ubuntu_rootfs_url=https://s3.amazonaws.com/spec.ccfc.min/ci-artifacts/disks/$(arch)/ubuntu-$(ubuntu_version).ext4
# ubuntu_ssh_key_url=https://s3.amazonaws.com/spec.ccfc.min/ci-artifacts/disks/$(arch)/ubuntu-$(ubuntu_version).id_rsa

# The below files are needed and can be downloaded from the internet
release_url=https://github.com/firecracker-microvm/firecracker/releases/download/$(firecracker_version)/firecracker-$(firecracker_version)-$(ARCH).tgz

testdata_objects = \
$(FC_TEST_DATA_PATH)/firecracker \
$(FC_TEST_BIN_PATH)/host-local \
$(FC_TEST_DATA_PATH)/jailer \
$(FC_TEST_DATA_PATH)/ltag \
$(FC_TEST_BIN_PATH)/ptp \
$(FC_TEST_DATA_PATH)/root-drive.img \
$(FC_TEST_DATA_PATH)/root-drive-with-ssh.img \
$(FC_TEST_DATA_PATH)/root-drive-ssh-key \
$(FC_TEST_BIN_PATH)/static \
$(FC_TEST_BIN_PATH)/tc-redirect-tap \
$(FC_TEST_DATA_PATH)/vmlinux 

testdata_dir = testdata/firecracker.tgz testdata/firecracker_spec-$(firecracker_version).yaml testdata/LICENSE testdata/NOTICE testdata/THIRD-PARTY

# --location is needed to follow redirects
curl = curl --location

GO_VERSION = $(shell go version | cut -c 14- | cut -d' ' -f1 | cut -d'.' -f1,2)
ifeq ($(GO_VERSION), $(filter $(GO_VERSION),1.14 1.15))
    define install_go
		cd .hack; GO111MODULE=on GOBIN=$(abspath $(FC_TEST_BIN_PATH)) go get $(1)@$(2) 
		cd .hack; GO111MODULE=on GOBIN=$(abspath $(FC_TEST_BIN_PATH)) go install $(1)
    endef
else
    define install_go
		GOBIN=$(abspath $(FC_TEST_BIN_PATH)) go install $(1)@$(2)
    endef
endif

all: build

test: all-tests

unit-tests: $(testdata_objects)
	DISABLE_ROOT_TESTS=$(DISABLE_ROOT_TESTS) go test -short ./... $(EXTRAGOARGS)

all-tests: $(testdata_objects)
	DISABLE_ROOT_TESTS=$(DISABLE_ROOT_TESTS) go test ./... $(EXTRAGOARGS)

generate build clean::
	go $@ $(EXTRAGOARGS)

clean::
	rm -fr build/

distclean: clean
	rm -rf $(testdata_objects)
	rm -f $(FC_TEST_DATA_PATH)/fc.stamp
	rm -rfv $(testdata_dir)
	docker volume rm -f $(CARGO_CACHE_VOLUME_NAME)

deps: $(testdata_objects)

$(FC_TEST_DATA_PATH)/vmlinux:
	$(curl) -o $@ $(kernel_url)

$(FC_TEST_DATA_PATH)/firecracker $(FC_TEST_DATA_PATH)/jailer: $(FC_TEST_DATA_PATH)/fc.stamp

# Download the pinned release version of firecracker and jailer version from github
$(FC_TEST_DATA_PATH)/fc.stamp:
	$(curl) ${release_url} | tar -xvzf - -C $(FC_TEST_DATA_PATH)
	mv $(FC_TEST_DATA_PATH)/release-$(firecracker_version)-$(arch)/firecracker-$(firecracker_version)-$(arch) $(FC_TEST_DATA_PATH)/firecracker
	mv $(FC_TEST_DATA_PATH)/release-$(firecracker_version)-$(arch)/jailer-$(firecracker_version)-$(arch) $(FC_TEST_DATA_PATH)/jailer
	rm -rf $(FC_TEST_DATA_PATH)/release-$(firecracker_version)-$(arch)
	touch $@

$(FC_TEST_DATA_PATH)/root-drive.img:
	$(curl) -o $@ https://s3.amazonaws.com/spec.ccfc.min/img/hello/fsfiles/hello-rootfs.ext4

# Download pre-built rootfs image and its ssh key from S3
$(FC_TEST_DATA_PATH)/root-drive-ssh-key $(FC_TEST_DATA_PATH)/root-drive-with-ssh.img: 
#	we will refactor below
# 	unsquashfs ubuntu-$ubuntu_version.squashfs.upstream
# 	ssh-keygen -f id_rsa -N ""
# 	cp -v id_rsa.pub squashfs-root/root/.ssh/authorized_keys
# 	mv -v id_rsa ./ubuntu-$ubuntu_version.id_rsa
# 	# create ext4 filesystem image
# 	sudo chown -R root:root squashfs-root
# 	truncate -s 1G ubuntu-$ubuntu_version.ext4
# 	sudo mkfs.ext4 -d squashfs-root -F ubuntu-$ubuntu_version.ext4


	$(curl) -o $(FC_TEST_DATA_PATH)/ubuntu-$(ubuntu_version).squashfs.upstream $(ubuntu_rootfs_url)
	unsquashfs $(FC_TEST_DATA_PATH)/ubuntu-$(ubuntu_version).squashfs.upstream -d $(FC_TEST_DATA_PATH)/ubuntu-rootfs
	ssh-keygen -f id_rsa -N ""
	cp -v id_rsa.pub $(FC_TEST_DATA_PATH)/ubuntu-rootfs/root/.ssh/authorized_keys
	mv -v id_rsa $(FC_TEST_DATA_PATH)/root-drive-ssh-key
# 	create ext4 filesystem image
	sudo chown -R root:root $(FC_TEST_DATA_PATH)/ubuntu-rootfs
	truncate -s 1G $(FC_TEST_DATA_PATH)/root-drive-with-ssh.img
	sudo mkfs.ext4 -d $(FC_TEST_DATA_PATH)/ubuntu-rootfs -F $(FC_TEST_DATA_PATH)/root-drive-with-ssh.img

# 	$(curl) -o $(FC_TEST_DATA_PATH)/root-drive-with-ssh.img $(ubuntu_rootfs_url)
# 	$(curl) -o $(FC_TEST_DATA_PATH)/root-drive-ssh-key $(ubuntu_ssh_key_url)

$(FC_TEST_BIN_PATH)/ptp:
	$(call install_go,github.com/containernetworking/plugins/plugins/main/ptp,v1.9.0)

$(FC_TEST_BIN_PATH)/host-local:
	$(call install_go,github.com/containernetworking/plugins/plugins/ipam/host-local,v1.9.0)

$(FC_TEST_BIN_PATH)/static:
	$(call install_go,github.com/containernetworking/plugins/plugins/ipam/static,v1.9.0)

$(FC_TEST_BIN_PATH)/tc-redirect-tap:
	$(call install_go,github.com/awslabs/tc-redirect-tap/cmd/tc-redirect-tap,v0.0.0-20250516183331-34bf829e9a5c)

$(FC_TEST_DATA_PATH)/ltag:
	$(call install_go,github.com/kunalkushwaha/ltag,v0.2.3)

# test-images builds firecracker and jailer from main branch of firecracker
# to test against HEAD of firecracker  
.PHONY: test-images
test-images: $(FIRECRACKER_BIN) $(JAILER_BIN)

$(FIRECRACKER_DIR):
	- git clone https://github.com/firecracker-microvm/firecracker.git -b firecracker-$(ci_version) $(FIRECRACKER_DIR)

$(FIRECRACKER_BIN) $(JAILER_BIN): $(FIRECRACKER_DIR)
	$(FIRECRACKER_DIR)/tools/devtool -y build --release
	cp $(FIRECRACKER_DIR)/build/cargo_target/$(FIRECRACKER_TARGET)/release/firecracker $(FIRECRACKER_BIN)
	cp $(FIRECRACKER_DIR)/build/cargo_target/$(FIRECRACKER_TARGET)/release/jailer $(JAILER_BIN)

.PHONY: firecracker-clean
firecracker-clean:
	- $(FIRECRACKER_DIR)/tools/devtool distclean
	- rm $(FIRECRACKER_BIN) $(JAILER_BIN)

lint: deps
	gofmt -s -l .
	$(FC_TEST_DATA_PATH)/bin/ltag -check -v -t .headers

.PHONY: all generate clean distclean build test unit-tests all-tests check-kvm
