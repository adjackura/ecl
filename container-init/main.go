package main

import (
	"io/ioutil"
	"log"
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
	cmd := exec.Command(path, args...)
	cmd.Env = []string{"PATH=/bin"}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Wait()
}

func runKublet() {
	for {
		kubletArgs := []string{
			"--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf",
			"--kubeconfig=/etc/kubernetes/kubelet.conf",
			"--config=/var/lib/kubelet/config.yaml",
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

	// Run containerd
	go func() {
		if err := run("/bin/containerd"); err != nil {
			logger.Println(err)
		}
		time.Sleep(1 * time.Second)
	}()

	// Run kublet
	go runKublet()

	select {}
}
