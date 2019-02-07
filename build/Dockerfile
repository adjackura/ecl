FROM ubuntu

RUN apt-get update && apt-get install -y \
  iptables \
  ca-certificates \
  socat \
  ebtables \
  iproute2 \
  util-linux \
  libseccomp2 \
  ethtool \
  curl

#RUN curl -L -O https://storage.googleapis.com/cri-containerd-release/cri-containerd-1.2.2.linux-amd64.tar.gz && \
#  tar --no-overwrite-dir -C / -xzf cri-containerd-1.2.2.linux-amd64.tar.gz && \
#  cp /usr/local/sbin/runc /runc

RUN mkdir -p /opt/bin && \
  cd /opt/bin && \
  curl -L --remote-name-all https://storage.googleapis.com/kubernetes-release/release/v1.13.2/bin/linux/amd64/{kubeadm,kubelet,kubectl} && \
  chmod +x kubeadm kubelet kubectl

RUN VERSION="v1.13.0" && \
  wget --quiet https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-$VERSION-linux-amd64.tar.gz && \
  tar zxvf crictl-$VERSION-linux-amd64.tar.gz -C /bin/

RUN CNI_VERSION="v0.7.4" && \
  mkdir -p /opt/cni/bin && \
  curl -L "https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/cni-plugins-amd64-${CNI_VERSION}.tgz" | tar -C /opt/cni/bin -xz && \