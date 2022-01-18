#! /bin/bash
set -ex

echo "AgileOS build status: installing dependencies"
apt-get update  
apt-get install -y dosfstools ca-certificates parted curl

PROJECT_ID=$(curl "http://metadata.google.internal/computeMetadata/v1/project/project-id" -H "Metadata-Flavor: Google")
OUTS=$(curl "http://metadata/computeMetadata/v1/instance/attributes/daisy-outs-path" -H "Metadata-Flavor: Google")
PACKAGE_BUCKET=$(curl "http://metadata/computeMetadata/v1/instance/attributes/package-bucket" -H "Metadata-Flavor: Google")
KERNEL_PACKAGE=$(curl "http://metadata/computeMetadata/v1/instance/attributes/kernel-package" -H "Metadata-Flavor: Google")
PACKAGES=$(curl "http://metadata/computeMetadata/v1/instance/attributes/packages" -H "Metadata-Flavor: Google")

echo "AgileOS build status: setting up the disk"
fallocate -l 10G disk.raw
parted -s disk.raw \
  mklabel gpt \
  mkpart ESP fat16 1MiB 131MiB \
  set 1 esp on \
  mkpart primary ext4 131MiB 100%
disk=$(losetup -Pf --show disk.raw)
mkfs.fat -F 16 -S 4096 ${disk}p1
mkfs.ext4 -b 4096 -F ${disk}p2

mkdir /mnt/p1
mount ${disk}p1 /mnt/p1
mkdir -p /mnt/p1/EFI/BOOT

mkdir /mnt/p2
mount ${disk}p2 /mnt/p2
mkdir /mnt/p2/dev -m 0755
mkdir /mnt/p2/sbin -m 0755
mkdir /mnt/p2/bin -m 0755
mkdir /mnt/p2/proc -m 0755
mkdir /mnt/p2/run -m 0755
mkdir /mnt/p2/root -m 0755
mkdir /mnt/p2/var -m 0755
mkdir /mnt/p2/sys -m 0755
mkdir /mnt/p2/tmp -m 1777
mkdir -p /mnt/p2/etc/ssl/certs
mkdir -p /mnt/p2/mnt/overlay

cp /etc/ssl/certs/ca-certificates.crt /mnt/p2/etc/ssl/certs/ca-certificates.crt

echo "AgileOS build status: pulling the kernel"
gsutil cp gs://${PACKAGE_BUCKET}/${KERNEL_PACKAGE} .
tar -C /mnt/p1/EFI/BOOT -xzvf $KERNEL_PACKAGE

echo "AgileOS build status: pulling the packages"
for PKG in $PACKAGES
do
  gsutil cp gs://${PACKAGE_BUCKET}/${PKG} .
  tar -C /mnt/p2 -xzvf $PKG
done

sync
umount /mnt/p1
umount /mnt/p2

tar -czvf disk.tar.gz disk.raw
gsutil cp disk.tar.gz ${OUTS}/disk.tar.gz
echo "AgileOS build finished"