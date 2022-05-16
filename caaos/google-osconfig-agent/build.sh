set -ex

cd caaos/google-osconfig-agent
mkdir -p pkgroot/p3
mkdir -p pkgroot/p4/opt/caaos/google-osconfig-agent

VERSION=20220510.00
git -c advice.detachedHead=false clone -b $VERSION https://github.com/GoogleCloudPlatform/osconfig.git
# git -c advice.detachedHead=false clone -b ostasks https://github.com/adjackura/osconfig.git
cd osconfig
CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}-agile" -o google_osconfig_agent
cd ..

cp ./osconfig/google_osconfig_agent pkgroot/p4/opt/caaos/google-osconfig-agent/google_osconfig_agent
cp -r etc/. pkgroot/p3/etc
mkdir -p /workspace/packages
tar -czvf /workspace/packages/google-osconfig-agent.tar.gz -C pkgroot .