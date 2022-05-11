set -ex

apt-get update 
apt-get install -y curl software-properties-common python3-jinja2
add-apt-repository -s -u "deb http://deb.debian.org/debian $(lsb_release -cs) main"
apt-get build-dep -y systemd
curl -sL https:/github.com/systemd/systemd/archive/refs/tags/v250.tar.gz | tar -xzvf -
cd systemd-250
meson build
meson compile -C build src/boot/efi/linuxx64.efi.stub
cp build/src/boot/efi/linuxx64.efi.stub /workspace/linuxx64.efi.stub