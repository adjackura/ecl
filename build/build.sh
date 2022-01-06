#! /bin/bash

echo "AgileOS build status: installing dependencies"
apt-get update  
DEBIAN_FRONTEND=noninteractive apt-get install -y \
  git-core \
  dosfstools \
  ca-certificates

set -e 

echo "AgileOS build status: cloning ecl"
git clone https://github.com/adjackura/ecl.git

echo "AgileOS build status: setting up the disk"
dd if=/dev/zero of=disk.img bs=1M count=1024
parted -s /dev/disk.img \
  mklabel gpt \
  mkpart ESP fat16 1MiB 131MiB \
  set 1 esp on \
  mkpart primary ext4 131MiB 100%
losetup -Pf --show disk.img 
mkfs.fat -F 16 -S 4096 /dev/loop0p1
mkfs.ext4 -b 4096 -F /dev/loop0p2
e2label /dev/loop0p2 root

mkdir /mnt/loop0p1
mount /dev/loop0p1 /mnt/loop0p1
mkdir /mnt/loop0p2
mount /dev/loop0p2 /mnt/loop0p2
mkdir /mnt/loop0p2/dev
mkdir /mnt/loop0p2/sbin
mkdir /mnt/loop0p2/bin
mkdir -p /mnt/loop0p2/etc/ssl/certs
mkdir -p /mnt/loop0p1/EFI/BOOT
cp /etc/ssl/certs/ca-certificates.crt /mnt/loop0p2/etc/ssl/certs/ca-certificates.crt
cp -r ecl/rootfs/* /mnt/loop0p2

echo "AgileOS build status: building the kernel"
# cp BOOTX64.EFI

echo "AgileOS build status: installing Go"
wget --quiet https://go.dev/dl/go1.17.5.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.17.5.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
export GOPATH=~/go
export GOCACHE=~/go/go-build
go version

echo "AgileOS build status: building init for host"
/mnt/sdb2/sbin/init

echo "AgileOS build status: building caaos"
/mnt/sdb2/bin/caaos

echo "AgileOS build status: building runc"
cp runc /mnt/sdb2/bin/

echo "AgileOS build status: building containerd"
cp bin/ctr /mnt/sdb2/bin/
cp bin/containerd /mnt/sdb2/bin/
cp bin/containerd-shim-runc-v2 /mnt/sdb2/bin/

echo "AgileOS build finished"