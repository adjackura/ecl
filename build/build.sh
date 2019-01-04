#! /bin/bash

echo "ECL build status: installing dependencies"
apt-get update  
DEBIAN_FRONTEND=noninteractive apt-get -y install build-essential git-core bison flex libelf-dev bc refind libseccomp-dev pkg-config dosfstools

echo "ECL build status: cloning ecl"
git clone https://github.com/adjackura/ecl.git -b kubernetes

echo "ECL build status: setting up the disk"
parted -s /dev/sdb \
  mklabel gpt \
  mkpart ESP fat32 1MiB 551MiB \
  set 1 esp on \
  mkpart primary ext4 551MiB 100%
sync
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
mkdir -p /mnt/sdb2/etc/ssl/certs
cp /etc/ssl/certs/ca-certificates.crt /mnt/sdb2/etc/ssl/certs/ca-certificates.crt

echo "ECL build status: building the kernel"
wget --quiet https://cdn.kernel.org/pub/linux/kernel/v4.x/linux-4.20.tar.xz
tar xf linux-4.20.tar.xz
cp ecl/linux/.config linux-4.20/.config
make -C linux-4.20 -j $(nproc)
cp linux-4.20/arch/x86_64/boot/bzImage /mnt/sdb2/

echo "ECL build status: setting up boot"
refind-install --usedefault /dev/sdb1
cp -r ecl/EFI /mnt/sdb1

echo "ECL build status: setting up container"
apt-get install -y \
     apt-transport-https \
     ca-certificates \
     curl \
     gnupg2 \
     software-properties-common
curl -fsSL https://download.docker.com/linux/debian/gpg | sudo apt-key add -
add-apt-repository \
   "deb [arch=amd64] https://download.docker.com/linux/debian \
   $(lsb_release -cs) \
   stable"
apt-get update
apt-get install docker-ce

cp -r ecl/container /mnt/sdb2
mkdir /mnt/sdb2/container/rootfs/bin
mkdir /mnt/sdb2/container/rootfs/sbin
mkdir /mnt/sdb2/container/rootfs/proc
mkdir /mnt/sdb2/container/rootfs/sys
mkdir /mnt/sdb2/container/rootfs/var
mkdir /mnt/sdb2/container/rootfs/dev/pts
mkdir /mnt/sdb2/container/rootfs/dev/shm
docker export $(docker create debian) | tar -C /mnt/sdb2/container/rootfs -xf -

echo "ECL build status: installing Go"
wget --quiet https://dl.google.com/go/go1.11.4.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.11.4.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
export GOPATH=~/go
go version

echo "ECL build status: building init for host"
pushd ecl/init
go get -d -v ./...
CGO_ENABLED=0 go build -ldflags '-s -w' -o /mnt/sdb2/sbin/init
popd

echo "ECL build status: building init for container"
pushd ecl/container-init
go get -d -v ./...
CGO_ENABLED=0 go build -ldflags '-s -w' -o /mnt/sdb2/container/rootfs/sbin/container-init
popd

echo "ECL build status: building runc for host and container"
go get -d -u github.com/opencontainers/runc
make -C $GOPATH/src/github.com/opencontainers/runc static
cp $GOPATH/src/github.com/opencontainers/runc/runc /mnt/sdb2/bin/
cp $GOPATH/src/github.com/opencontainers/runc/runc /mnt/sdb2/container/rootfs/bin/

echo "ECL build status: building containerd for container"
go get -d -u github.com/containerd/containerd
make -C $GOPATH/src/github.com/containerd/containerd EXTRA_FLAGS="-buildmode pie" EXTRA_LDFLAGS='-s -w -extldflags "-fno-PIC -static"' BUILDTAGS="no_btrfs netgo osusergo static_build"
cp $GOPATH/src/github.com/containerd/containerd/bin/ctr /mnt/sdb2/container/rootfs/bin/
cp $GOPATH/src/github.com/containerd/containerd/bin/containerd /mnt/sdb2/container/rootfs/bin/
cp $GOPATH/src/github.com/containerd/containerd/bin/containerd-shim /mnt/sdb2/container/rootfs/bin/

echo "ECL build status: building Kubernetes for container"
go get -d k8s.io/kubernetes
make -C $GOPATH/src/k8s.io/kubernetes WHAT=cmd/kubeadm
make -C $GOPATH/src/k8s.io/kubernetes WHAT=cmd/kubectl
make -C $GOPATH/src/k8s.io/kubernetes WHAT=cmd/kubelet GOLDFLAGS='-w -extldflags "-static"' GOFLAGS='-tags=osusergo'
cp $GOPATH/src/k8s.io/kubernetes/_output/bin/kube* /mnt/sdb2/container/rootfs/bin/

echo "ECL build status: pulling crictl for container"
VERSION="v1.13.0"
wget --quiet https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-$VERSION-linux-amd64.tar.gz
sudo tar zxvf crictl-$VERSION-linux-amd64.tar.gz -C /mnt/sdb2/container/rootfs/bin/

echo "ECL build status: pulling cni plugins for container"
CNI_VERSION="v0.7.4"
mkdir -p /mnt/sdb2/container/rootfs/opt/cni/bin
curl -L "https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/cni-plugins-amd64-${CNI_VERSION}.tgz" | tar -C /mnt/sdb2/container/rootfs/opt/cni/bin -xz

echo "ECL build finished"