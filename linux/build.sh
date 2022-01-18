set -ex

apt-get update && apt-get install -y curl gnupg2 software-properties-common
curl -sSL https://apt.llvm.org/llvm-snapshot.gpg.key | apt-key add -
add-apt-repository -u "deb [arch=amd64] http://apt.llvm.org/$(lsb_release -cs)/ llvm-toolchain-$(lsb_release -cs)-13 main"
add-apt-repository -s -u "deb http://deb.debian.org/debian $(lsb_release -cs) main"
apt-get install -y \
  clang-13 \
  llvm-13 \
  lld-13 \
  build-essential \
  bison \
  flex \
  libelf-dev \
  bc \
  python3-jinja2 \
  liblz4-tool
apt-get build-dep -y systemd

KERNEL_VERSION='5.15.14'
curl -s https://cdn.kernel.org/pub/linux/kernel/v5.x/linux-${KERNEL_VERSION}.tar.xz | tar -Jxf -
cp linux/.config linux-${KERNEL_VERSION}/.config 
make -C linux-${KERNEL_VERSION} -j $(nproc) \
  CC=clang-13 \
  LD=ld.lld-13 \
  AR=llvm-ar-13 \
  NM=llvm-nm-13 \
  STRIP=llvm-strip-13 \
  OBJCOPY=llvm-objcopy-13 \
  OBJDUMP=llvm-objdump-13 \
  READELF=llvm-readelf-13 \
  HOSTCC=clang-13 \
  HOSTCXX=clang++-13 \
  HOSTAR=llvm-ar-13 \
  HOSTLD=ld.lld-13

curl -sL https:/github.com/systemd/systemd/archive/refs/tags/v250.tar.gz | tar -xzvf -
pushd systemd-250
meson build
meson compile -C build src/boot/efi/linuxx64.efi.stub
popd

mkdir pkgroot
objcopy \
  --add-section .osrel="init/etc/os-release" --change-section-vma .osrel=0x20000 \
  --add-section .cmdline="linux/cmdline" --change-section-vma .cmdline=0x30000 \
  --add-section .linux="linux-${KERNEL_VERSION}/arch/x86_64/boot/bzImage" --change-section-vma .linux=0x40000 \
  systemd-250/build/src/boot/efi/linuxx64.efi.stub pkgroot/BOOTX64.EFI
  
tar -czvf kernel.tar.gz -C pkgroot .