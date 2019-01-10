package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	logger = log.New(os.Stdout, "[container]: ", log.LstdFlags|log.Lmicroseconds|log.Lshortfile)
)

type systemService struct {
	name, desc, path string
	args             []string
	running          bool
	mx               sync.RWMutex
}

func run(path string, args ...string) error {
	logger.Printf("Running command %q with args %q", path, args)
	cmd := exec.Command(path, args...)
	cmd.Env = []string{"PATH=/bin"}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Wait()
}

type attributesJSON struct {
	Master             string `json:"master"`
	Token              string `json:"token"`
	DiscoveryTokenHash string `json:"discovery-token-ca-cert-hash"`
	Args               string `json:"args"`
}

func getMetadata() (*attributesJSON, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/attributes?recursive=true&alt=json", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Metadata-Flavor", "Google")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	md, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	var metadata attributesJSON
	return &metadata, json.Unmarshal(md, &metadata)
}

func runKubeadm() {
	md, err := getMetadata()
	if err != nil {
		logger.Println(err)
	}

	kubeadmArgs := []string{
		"join",
		md.Master,
		"--token",
		md.Token,
		"--discovery-token-ca-cert-hash",
		md.DiscoveryTokenHash,
	}

	if md.Args != "" {
		kubeadmArgs = append(kubeadmArgs, strings.Split(md.Args, ";")...)
	}

	if err := run("/bin/kubeadm", kubeadmArgs...); err != nil {
		logger.Println(err)
	}
}

func runKublet() {
	for {
		kubletArgs := []string{
			"--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf",
			"--kubeconfig=/etc/kubernetes/kubelet.conf",
			"--config=/var/lib/kubelet/config.yaml",
			"--container-runtime=remote",
			"--container-runtime-endpoint=unix:///run/containerd/containerd.sock",
			"--fail-swap-on=false",
			"-v 3",
		}

		d, err := ioutil.ReadFile("/var/lib/kubelet/kubeadm-flags.env")
		if err != nil {
			logger.Println(err)
		} else {
			// KUBELET_KUBEADM_ARGS=--cgroup-driver=cgroupfs --cni-bin-dir=/opt/cni/bin --cni-conf-dir=/etc/cni/net.d --network-plugin=cni
			out := strings.Replace(string(d), "KUBELET_KUBEADM_ARGS=", "", 1)
			kubletArgs = append(kubletArgs, strings.Split(out, " ")...)
		}

		if err := run("/bin/kubelet", kubletArgs...); err != nil {
			logger.Println(err)
		}
		time.Sleep(1 * time.Second)
	}
}

func main() {
	logger.Println("Starting container init...")

	logger.Println("Enable ip forwarding")
	if err := ioutil.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644); err != nil {
		logger.Println(err)
	}

	// Run containerd
	go func() {
		for {
			if err := run("/bin/containerd"); err != nil {
				logger.Println(err)
			}
			time.Sleep(1 * time.Second)
		}
	}()

	// Run kublet
	go runKublet()
	runKubeadm()

	select {}
}
