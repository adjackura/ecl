#! /bin/bash
set -ex

echo "AgileOS build status: installing dependencies"
apt-get update  
apt-get install -y dosfstools ca-certificates parted curl sbsigntool cryptsetup-bin binutils

PROJECT_ID=$(curl "http://metadata.google.internal/computeMetadata/v1/project/project-id" -H "Metadata-Flavor: Google")
OUTS=$(curl "http://metadata/computeMetadata/v1/instance/attributes/daisy-outs-path" -H "Metadata-Flavor: Google")
SOURCES=$(curl "http://metadata/computeMetadata/v1/instance/attributes/daisy-sources-path" -H "Metadata-Flavor: Google")
KERNEL_PACKAGE=$(curl "http://metadata/computeMetadata/v1/instance/attributes/kernel-package" -H "Metadata-Flavor: Google")
PACKAGES=$(curl "http://metadata/computeMetadata/v1/instance/attributes/packages" -H "Metadata-Flavor: Google")

echo "AgileOS build status: setting up the disk"
fallocate -l 10G disk.raw
parted -s disk.raw \
  mklabel gpt \
  mkpart ESP fat16 1MiB 131MiB \
  set 1 esp on \
  mkpart primary ext4 131MiB 231MiB \
  mkpart primary ext4 231MiB 531MiB \
  mkpart primary ext4 531MiB 100%
disk=$(losetup -Pf --show disk.raw)
mkfs.fat -F 16 -S 4096 ${disk}p1
mkfs.ext4 -b 4096 -F ${disk}p3
mkfs.ext4 -b 4096 -F ${disk}p4

mkdir /mnt/p1
mount ${disk}p1 /mnt/p1
mkdir -p /mnt/p1/EFI/BOOT

mkdir /mnt/p3
mount ${disk}p3 /mnt/p3
mkdir /mnt/p3/bin -m 0755
mkdir /mnt/p3/dev -m 0755
mkdir /mnt/p3/mnt -m 0755
mkdir /mnt/p3/opt -m 0755
mkdir /mnt/p3/proc -m 0755
mkdir /mnt/p3/run -m 0755
mkdir /mnt/p3/sbin -m 0755
mkdir /mnt/p3/sys -m 0755
mkdir /mnt/p3/tmp -m 1777
mkdir /mnt/p3/var -m 0755
mkdir -p /mnt/p3/etc/ssl/certs

mkdir /mnt/p4
mount ${disk}p4 /mnt/p4
mkdir /mnt/p4/var -m 0755
mkdir /mnt/p4/opt -m 0755

cp /etc/ssl/certs/ca-certificates.crt /mnt/p3/etc/ssl/certs/ca-certificates.crt

echo "AgileOS build status: pulling the packages"
for PKG in $PACKAGES
do
  gsutil cp ${SOURCES}/packages/${PKG} .
  tar -C /mnt -xzvf $PKG
done

cp /mnt/p3/etc/os-release .
sync
umount /mnt/p3
umount /mnt/p4

# Setup dm-verity for root
veritysetup --data-block-size=4096 --hash-block-size=4096 --hash=sha256 --format=1 format ${disk}p3 ${disk}p2 > veritysetupout
digest=$(cat veritysetupout | grep "Root" | awk '{print $NF}')
salt=$(cat veritysetupout | grep "Salt" | awk '{print $NF}')
blocks=$(cat veritysetupout | grep "Data blocks" | awk '{print $NF}')
dmmod="1 /dev/sda3 /dev/sda2 4096 4096 ${blocks} 1 sha256 ${digest} ${salt}"
echo $dmmod

echo "AgileOS build status: pulling the kernel"
gsutil cp ${SOURCES}/packages/${KERNEL_PACKAGE} .
tar -xzvf $KERNEL_PACKAGE

echo "AgileOS build status: building unified kernel"
gsutil cp  "${SOURCES}/linuxx64.efi.stub" .

sectors=$((($blocks * 4096) / 512))
echo "ip=dhcp console=ttyS0,115200n8 loglevel=4 elevator=noop printk.devkmsg=on dm-mod.create=\"root,,,ro,0 ${sectors} verity ${dmmod}\" root=/dev/dm-0 ro" > cmdline
objcopy \
  --add-section .osrel="os-release" --change-section-vma .osrel=0x20000 \
  --add-section .cmdline="cmdline" --change-section-vma .cmdline=0x30000 \
  --add-section .linux="bzImage" --change-section-vma .linux=0x40000 \
  linuxx64.efi.stub /mnt/p1/EFI/BOOT/BOOTX64.EFI

echo "AgileOS build status: signing the kernel"
gsutil cp  "${SOURCES}/certs/db.*" .
sbsign --key db.key --cert db.crt --output /mnt/p1/EFI/BOOT/BOOTX64.EFI /mnt/p1/EFI/BOOT/BOOTX64.EFI

echo "AgileOS build status: compressing the image"
sync
umount /mnt/p1
tar -czvf disk.tar.gz disk.raw

echo "AgileOS build status: uploading the image"
gsutil cp disk.tar.gz ${OUTS}/disk.tar.gz
echo "AgileOS build finished"