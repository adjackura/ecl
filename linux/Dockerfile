FROM launcher.gcr.io/google/debian11

RUN apt-get update && apt-get install -y wget gnupg2 
RUN echo "deb http://apt.llvm.org/bullseye/ llvm-toolchain-bullseye-13 main" >> /etc/apt/sources.list \
    && wget -O - https://apt.llvm.org/llvm-snapshot.gpg.key | apt-key add - 
RUN apt-get update && apt-get install -y \
    clang-13 \
    build-essential \
    bison \
    flex \
    libelf-dev \
    bc \
    software-properties-common \ 
    systemd \
    liblz4-tool

WORKDIR /workspace/linux
RUN wget --quiet https://cdn.kernel.org/pub/linux/kernel/v5.x/linux-5.15.tar.xz \
    && tar xf linux-5.15.tar.xz 
COPY linux/.config linux-5.15/.config 
RUN ls 
RUN ls linux-5.15
RUN make -C linux-5.15 -j $(nproc) CC=clang-13

COPY linux/os-release linux/cmdline ./
RUN objcopy \
    --add-section .osrel="os-release" --change-section-vma .osrel=0x20000 \
    --add-section .cmdline="cmdline" --change-section-vma .cmdline=0x30000 \
    --add-section .linux="linux-5.15/arch/x86_64/boot/bzImage" --change-section-vma .linux=0x40000 \
    /usr/lib/systemd/boot/efi/linuxx64.efi.stub /BOOTX64.EFI

FROM scratch
COPY --from=0 BOOTX64.EFI /