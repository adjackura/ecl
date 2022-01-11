set -ex

cd services/otelopscol
mkdir -p pkgroot/bin

git clone https://github.com/GoogleCloudPlatform/opentelemetry-operations-collector.git
cd opentelemetry-operations-collector
git -c advice.detachedHead=false checkout d2c97eb
CGO_ENABLED=0 go build -ldflags '-s -w' -o otelopscol ./cmd/otelopscol 
cd ..

cp ./opentelemetry-operations-collector/otelopscol pkgroot/bin/otelopscol
cp -r etc pkgroot/etc
tar -czvf otelopscol.tar.gz -C pkgroot .