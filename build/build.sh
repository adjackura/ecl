#! /bin/bash

apt-get update  
apt-get -y install build-essential git-core bison flex libelf-dev bc refind libseccomp-dev pkg-config dosfstools

git clone https://github.com/adjackura/ecl.git
git clone https://kernel.googlesource.com/pub/scm/linux/kernel/git/torvalds/linux
cp ecl/linux/.config linux/.config

wget https://dl.google.com/go/go1.11.1.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.11.4.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$(go env GOPATH)

# Setup the disk
parted -s /dev/sdb mklabel gpt mkpart ESP fat32 1MiB 551MiB set 1 esp on mkpart primary ext4 551MiB 100%
mkfs.vfat -F32 /dev/sdb1
mkfs.ext4 -F /dev/sdb2
e2label /dev/sdb2 root

mkdir /mnt/sdb1
mount /dev/sdb1 /mnt/sdb1
mkdir /mnt/sdb2
mount /dev/sdb2 /mnt/sdb2
mkdir /mnt/sdb2/dev
mkdir /mnt/sdb2/sbin
mkdir /mnt/sdb2/bin
cp -r ecl/etc /mnt/sdb2
mkdir -p /mnt/sdb2/etc/ssl/certs
cp /etc/ssl/certs/ca-certificates.crt /mnt/sdb2/etc/ssl/certs/ca-certificates.crt

# Build the kernel
make -C linux olddefconfig
make -C linux -j 4
cp linux/arch/x86_64/boot/bzImage /mnt/sdb2/

# Setup boot
refind-install --usedefault /dev/sdb1
cp -r ecl/EFI /mnt/sdb1

# Build init
go get -d -u github.com/adjackura/ecl/init
CGO_ENABLED=0 go build -ldflags '-s -w' -o /mnt/sdb2/sbin/init github.com/adjackura/ecl/init

# Build containerd
go get -d -u github.com/containerd/containerd
make -C $GOPATH/src/github.com/containerd/containerd EXTRA_FLAGS="-buildmode pie" EXTRA_LDFLAGS='-s -w -extldflags "-fno-PIC -static"' BUILDTAGS="no_btrfs netgo osusergo static_build"
cp $GOPATH/src/github.com/containerd/containerd/bin/ctr /mnt/sdb2/bin/ctr
cp $GOPATH/src/github.com/containerd/containerd/bin/containerd /mnt/sdb2/bin/containerd
cp $GOPATH/src/github.com/containerd/containerd/bin/containerd-shim /mnt/sdb2/bin/containerd-shim

# Build runc
go get -d -u github.com/opencontainers/runc
make -C $GOPATH/src/github.com/opencontainers/runc static
cp $GOPATH/src/github.com/opencontainers/runc/runc /mnt/sdb2/bin/runc

# Kubernetes
go get -d k8s.io/kubernetes
make -C $GOPATH/src/k8s.io/kubernetes WHAT=cmd/kubeadm
make -C $GOPATH/src/k8s.io/kubernetes WHAT=cmd/kubectl
make -C $GOPATH/src/k8s.io/kubernetes WHAT=cmd/kubelet GOLDFLAGS='-w -extldflags "-static"' GOFLAGS='-tags=osusergo'
cp $GOPATH/src/k8s.io/kubernetes/_output/bin/kube* /mnt/sdb2/bin/