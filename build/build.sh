#! /bin/bash

echo "AgileOS build status: installing dependencies"

deb http://apt.llvm.org/stretch/ llvm-toolchain-stretch main
deb-src http://apt.llvm.org/stretch/ llvm-toolchain-stretch main

apt-get update  
DEBIAN_FRONTEND=noninteractive apt-get install -y \
  wget \
  curl \
  build-essential \
  git-core \
  bison \
  flex \
  libelf-dev \
  bc \
  refind \
  libseccomp-dev \
  pkg-config \
  dosfstools \
  apt-transport-https \
  gnupg2 \
  software-properties-common \
  liblz4-tool \
  ca-certificates

set -e 

echo "AgileOS build status: cloning ecl"
git clone https://github.com/adjackura/ecl.git

echo "AgileOS build status: setting up the disk"
parted -s /dev/sdb \
  mklabel gpt \
  mkpart ESP fat16 1MiB 20MiB \
  set 1 esp on \
  mkpart primary ext4 20MiB 100%
sync
mkfs.vfat -S 4096 /dev/sdb1
mkfs.ext4 -b 4096 -F /dev/sdb2
e2label /dev/sdb2 root

mkdir /mnt/sdb1
mount /dev/sdb1 /mnt/sdb1
mkdir /mnt/sdb2
mount /dev/sdb2 /mnt/sdb2
mkdir /mnt/sdb2/dev
mkdir /mnt/sdb2/sbin
mkdir /mnt/sdb2/bin
mkdir -p /mnt/sdb2/etc/ssl/certs
cp /etc/ssl/certs/ca-certificates.crt /mnt/sdb2/etc/ssl/certs/ca-certificates.crt
cp -r ecl/rootfs/* /mnt/sdb2

echo "AgileOS build status: setting up boot"
refind-install --usedefault /dev/sdb1
cp -r ecl/EFI /mnt/sdb1

echo "AgileOS build status: building the kernel"
wget --quiet https://cdn.kernel.org/pub/linux/kernel/v5.x/linux-5.15.tar.xz
tar xf linux-5.15.tar.xz
cp ecl/linux/.config linux-5.15/.config
make -C linux-5.15 -j $(nproc)
cp linux-5.15/arch/x86_64/boot/bzImage /mnt/sdb1/

echo "AgileOS build status: installing Go"
wget --quiet https://go.dev/dl/go1.17.5.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.17.5.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
export GOPATH=~/go
go version

echo "AgileOS build status: building init for host"
pushd ecl/init
go get -d -v ./...
CGO_ENABLED=0 go build -ldflags '-s -w' -o /mnt/sdb2/sbin/init
popd

echo "AgileOS build status: building caaos"
pushd services/caaos
go get -d -v ./...
go build -tags 'netgo osusergo' -buildmode pie -ldflags '-s -w -extldflags "-static"' -o /mnt/sdb2/bin/caaos
popd

echo "AgileOS build status: building runc"
go get -d -u github.com/opencontainers/runc
make -C $GOPATH/src/github.com/opencontainers/runc static
cp $GOPATH/src/github.com/opencontainers/runc/runc /mnt/sdb2/opt/bin/

echo "AgileOS build status: building containerd"
go get -d -u github.com/containerd/containerd
make -C $GOPATH/src/github.com/containerd/containerd EXTRA_FLAGS="-buildmode pie" EXTRA_LDFLAGS='-s -w -extldflags "-fno-PIC -static"' BUILDTAGS="no_btrfs netgo osusergo static_build"
cp $GOPATH/src/github.com/containerd/containerd/bin/ctr /mnt/sdb2/bin/
cp $GOPATH/src/github.com/containerd/containerd/bin/containerd /mnt/sdb2/bin/
cp $GOPATH/src/github.com/containerd/containerd/bin/containerd-shim /mnt/sdb2/bin/

echo "AgileOS build finished"