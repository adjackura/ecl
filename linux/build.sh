apt-get update && apt-get install -y curl gnupg2 software-properties-common
curl -sSL https://apt.llvm.org/llvm-snapshot.gpg.key | apt-key add -
add-apt-repository "deb [arch=amd64] http://apt.llvm.org/$(lsb_release -cs)/ llvm-toolchain-$(lsb_release -cs)-13 main"
apt-get update && apt-get install -y \
  clang-13 \
  llvm-13 \
  lld-13 \
  build-essential \
  bison \
  flex \
  libelf-dev \
  bc \
  systemd \
  liblz4-tool

curl -s https://cdn.kernel.org/pub/linux/kernel/v5.x/linux-5.15.9.tar.xz | tar -Jxf -
cp linux/.config linux-5.15.9/.config 
make -C linux-5.15.9 -j $(nproc) \
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

mkdir pkgroot
objcopy \
  --add-section .osrel="linux/os-release" --change-section-vma .osrel=0x20000 \
  --add-section .cmdline="linux/cmdline" --change-section-vma .cmdline=0x30000 \
  --add-section .linux="linux-5.15.9/arch/x86_64/boot/bzImage" --change-section-vma .linux=0x40000 \
  /usr/lib/systemd/boot/efi/linuxx64.efi.stub pkgroot/BOOTX64.EFI
  
tar -czvf kernel.tar.gz -C pkgroot .