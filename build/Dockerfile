FROM ubuntu

RUN apt-get update && apt-get install -y \
  iptables \
  ca-certificates \
  socat \
  ebtables \
  iproute2 \
  util-linux \
  libseccomp2 \
  wget \
  curl

RUN curl -L --remote-name-all https://storage.googleapis.com/cri-containerd-release/cri-containerd-1.2.2.linux-amd64.tar.gz && \
  tar --no-overwrite-dir -C / -xzf cri-containerd-1.2.2.linux-amd64.tar.gz

RUN mkdir -p /opt/bin && \
  cd /opt/bin && \
  curl -L --remote-name-all https://storage.googleapis.com/kubernetes-release/release/1.13/bin/linux/amd64/{kubeadm,kubelet,kubectl} && \
  chmod +x kubeadm kubelet kubectl