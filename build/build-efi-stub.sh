set -ex

apt-get update 
apt-get install -y curl lsb-release python3-jinja2
echo "deb-src http://deb.debian.org/debian $(lsb_release -cs) main" >> /etc/apt/sources.list
apt-get update 
apt-get build-dep -y systemd

curl -sL https:/github.com/systemd/systemd/archive/refs/tags/v250.tar.gz | tar -xzvf -
cd systemd-250
meson build
meson compile -C build src/boot/efi/linuxx64.efi.stub
cp build/src/boot/efi/linuxx64.efi.stub /workspace/linuxx64.efi.stub