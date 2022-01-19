set -ex

apk --no-cache add ca-certificates

cd caaos/google-osconfig-agent
mkdir -p pkgroot/p2
mkdir -p pkgroot/p3/opt/caaos/google-osconfig-agent

VERSION=20220107.00
#git -c advice.detachedHead=false clone -b $VERSION https://github.com/GoogleCloudPlatform/osconfig.git
git clone https://github.com/adjackura/osconfig.git
cd osconfig
CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o google_osconfig_agent
cd ..

cp ./osconfig/google_osconfig_agent pkgroot/p3/opt/caaos/google-osconfig-agent/google_osconfig_agent
cp -r etc/. pkgroot/p2/etc
tar -czvf google-osconfig-agent.tar.gz -C pkgroot .