#! /bin/bash

apt-get update  
apt-get -y install build-essential git-core bison flex libelf-dev bc refind libseccomp-dev pkg-config

git clone https://github.com/adjackura/caaos.git
git clone https://kernel.googlesource.com/pub/scm/linux/kernel/git/torvalds/linux
cp caaos/kernel/.config linux/.config

wget https://dl.google.com/go/go1.10.3.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.10.3.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$(go env GOPATH)

# Setup the disk
parted -s /dev/sdb mklabel gpt mkpart ESP fat32 1MiB 551MiB set 1 esp on mkpart primary ext4 551MiB 100%
mkfs.vfat -F32 /dev/sdb1
mkfs.ext4 /dev/sdb2
e2label /dev/sdb2 root

mkdir /mnt/sdb1
mount /dev/sdb1 /mnt/sdb1
mkdir /mnt/sdb2
mount /dev/sdb2 /mnt/sdb2
mkdir /mnt/sdb2/dev
mkdir /mnt/sdb2/sbin
mkdir /mnt/sdb2/bin
cp -r caaos/etc /mnt/sdb2
cp /etc/ssl/certs/ca-certificates.crt /mnt/sdb2/etc/ssl/certs/ca-certificates.crt

# Build the kernel
make -C linux olddefconfig
make -C linux -j 4
cp linux/arch/x86_64/boot/bzImage /mnt/sdb1/

# Setup boot
refind-install --usedefault /dev/sdb1
cp caaos/refind.conf /mnt/sdb1/EFI/BOOT/refind.conf
mkdir -p /mnt/sdb1/EFI/Google/gsetup
echo '\EFI\Boot\bootx64.efi' > '/mnt/sdb1/EFI/Google/gsetup/Boot'

# Build init
go get -d github.com/adjackura/caaos/init
cd $GOPATH/src/github.com/adjackura/caaos/init
go build -buildmode pie -ldflags '-extldflags "-fno-PIC -static"'
cp init /mnt/sdb2/sbin/init

# Build caaos
go get -d github.com/adjackura/caaos/services/caaos
go build -buildmode pie -ldflags '-extldflags "-fno-PIC -static"' -o /mnt/sdb2/bin/caaos github.com/adjackura/caaos/services/caaos

# Build containerd
go get -d github.com/containerd/containerd
cd $GOPATH/src/github.com/containerd/containerd
make EXTRA_FLAGS="-buildmode pie" EXTRA_LDFLAGS='-extldflags "-fno-PIC -static"' BUILDTAGS="no_cri no_btrfs netgo osusergo static_build"  
cp bin/ctr /mnt/sdb2/bin/ctr
cp bin/containerd /mnt/sdb2/bin/containerd

# Build runc
go get -d github.com/opencontainers/runc
cd $GOPATH/src/github.com/opencontainers/runc
make static
cp runc /mnt/sdb2/bin/runc