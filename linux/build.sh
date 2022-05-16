set -ex

apt-get update 
apt-get install -y curl gnupg2 lsb-release
echo "deb [arch=amd64] http://apt.llvm.org/$(lsb_release -cs)/ llvm-toolchain-$(lsb_release -cs)-13 main" >> /etc/apt/sources.list
curl -sSL https://apt.llvm.org/llvm-snapshot.gpg.key | apt-key add -

apt-get update 
apt-get install -y \
  clang-13 \
  llvm-13 \
  lld-13 \
  build-essential \
  bison \
  flex \
  libelf-dev \
  bc \
  liblz4-tool

KERNEL_VERSION='5.17.6'
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

mkdir pkgroot
cp "linux-${KERNEL_VERSION}/arch/x86_64/boot/bzImage" pkgroot/
  
mkdir -p /workspace/packages
tar -czvf /workspace/packages/kernel.tar.gz -C pkgroot .