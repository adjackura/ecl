set -ex

git -c advice.detachedHead=false clone https://github.com/GoogleCloudPlatform/compute-daisy.git
cd compute-daisy/cli
CGO_ENABLED=0 go build -o /workspace/daisy