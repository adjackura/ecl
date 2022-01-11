set -ex

apk add --no-cache libseccomp-static libseccomp-dev pkgconfig

cd services/google-osconfig-agent
mkdir -p pkgroot/bin

VERSION=20220107.00
#git -c advice.detachedHead=false clone -b $VERSION https://github.com/GoogleCloudPlatform/osconfig.git
git clone https://github.com/adjackura/osconfig.git
cd osconfig
CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o google_osconfig_agent
cd ..

cp ./osconfig/google_osconfig_agent pkgroot/bin/google_osconfig_agent
cp -r etc pkgroot/etc
tar -czvf google-osconfig-agent.tar.gz -C pkgroot .