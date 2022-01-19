package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	metadataURL  = "http://metadata.google.internal/computeMetadata/v1/instance/attributes"
	metadataHang = "/?recursive=true&alt=json&wait_for_change=true&timeout_sec=120&last_etag="
	defaultEtag  = "NONE"
)

var (
	defaultTimeout = 130 * time.Second
	etag           = defaultEtag
	writerChan     = make(chan string, 10)

	logger = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds|log.Lshortfile)
)

type attributesJSON struct {
	ContainerRef      string `json:"container-ref"`
	ContainerSpec     string `json:"container-spec"`
	OverwriteDefaults bool   `json:"overwrite-defaults,string"`
	StopOnExit        bool   `json:"stop-on-exit,string"`
}

func updateEtag(resp *http.Response) bool {
	oldEtag := etag
	etag = resp.Header.Get("etag")
	if etag == "" {
		etag = defaultEtag
	}
	return etag == oldEtag
}

func watchMetadata(ctx context.Context) (*attributesJSON, error) {
	client := &http.Client{
		Timeout: defaultTimeout,
	}

	req, err := http.NewRequest("GET", metadataURL+metadataHang+etag, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Metadata-Flavor", "Google")
	req = req.WithContext(ctx)

	for {
		resp, err := client.Do(req)
		// Don't return error on a canceled context.
		if err != nil && ctx.Err() != nil {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}

		// Only return metadata on updated etag.
		if updateEtag(resp) {
			continue
		}

		md, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		var attr attributesJSON
		return &attr, json.Unmarshal(md, &attr)
	}
}

type consoleWriter struct {
	name string
}

func (w *consoleWriter) Write(b []byte) (int, error) {
	var msg string
	for _, b := range bytes.Split(bytes.TrimRight(b, "\n"), []byte("\n")) {
		msg += fmt.Sprintf("[%s] %s\n", w.name, b)
	}
	writerChan <- msg
	return len(b), nil
}

func withSpecFromBytes(p []byte, clear bool) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *oci.Spec) error {
		if clear {
			*s = oci.Spec{} // make sure spec is cleared.
		}
		if err := json.Unmarshal(p, s); err != nil {
			return err
		}
		return nil
	}
}

func runContainer(ctx context.Context, container containerd.Container) error {
	s, _ := container.Spec(ctx)
	d, _ := json.Marshal(s)
	fmt.Printf("%q container spec: %s\n", container.ID(), string(d))

	// create a new task
	w := &consoleWriter{name: container.ID()}
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStreams(os.Stdin, w, w)))
	if err != nil {
		return err
	}

	// Setup wait channel
	statusC, err := task.Wait(ctx)
	if err != nil {
		return err
	}

	// start the task
	logger.Printf("Starting task for container %q", container.ID())
	if err := task.Start(ctx); err != nil {
		return err
	}

	// wait for the task to exit and get the exit status
	logger.Printf("Waiting for %q...", container.ID())
	status := <-statusC
	code, _, err := status.Result()
	if err != nil {
		return err
	}

	logger.Printf("Return code for %q: %d", container.ID(), code)

	if _, err := task.Delete(ctx); err != nil {
		logger.Println(err)
	}

	return nil
}

func runContainerFromImage(ctx context.Context, client *containerd.Client, ref string, spec string, clear bool) error {
	logger.Printf("Recieved request to run %q", ref)

	var img containerd.Image
	imgs, err := client.ListImages(ctx)
	if err != nil {
		return fmt.Errorf("error listing images: %v", err)
	}
	for _, i := range imgs {
		if i.Name() == ref {
			logger.Println("Image found in local registry")
			img = i
		}
	}

	if img == nil {
		logger.Println("Image not found in local registry, pulling now")
		img, err = client.Pull(ctx, ref, containerd.WithPullUnpack)
		if err != nil {
			return fmt.Errorf("error pulling image: %v", err)
		}
	}

	id := fmt.Sprintf("%d", time.Now().Unix())
	logger.Println("Creating container")
	container, err := client.NewContainer(
		ctx,
		id,
		containerd.WithNewSnapshot(id, img),
		containerd.WithNewSpec(oci.WithImageConfig(img), withSpecFromBytes([]byte(spec), clear)),
	)
	if err != nil {
		return err
	}
	defer container.Delete(ctx, containerd.WithSnapshotCleanup)

	return runContainer(ctx, container)
}

type caaosService struct {
	ID, Delay                                                           string
	FullSpec                                                            bool
	WithPrivileged, WithAllDevicesAllowed, WithHostDevices, WithNetHost bool
	Mounts                                                              []specs.Mount
	OCISpec                                                             json.RawMessage
}

func withHostCACertsFile(_ context.Context, _ oci.Client, _ *containers.Container, s *oci.Spec) error {
	s.Mounts = append(s.Mounts, specs.Mount{
		Destination: "/etc/ssl/certs/ca-certificates.crt",
		Type:        "bind",
		Source:      "/etc/ssl/certs/ca-certificates.crt",
		Options:     []string{"rbind", "ro"},
	})
	return nil
}

func (s *caaosService) getContainer(ctx context.Context, client *containerd.Client) (containerd.Container, error) {
	cntrs, err := client.Containers(ctx)
	if err != nil {
		return nil, err
	}
	for _, cntr := range cntrs {
		if cntr.ID() == s.ID {
			return cntr, nil
		}
	}

	logger.Printf("Creating container %q", s.ID)
	specOpts := []oci.SpecOpts{
		oci.WithDefaultSpec(),
		oci.WithDefaultUnixDevices,
		withSpecFromBytes([]byte(s.OCISpec), s.FullSpec),
		oci.WithMounts(s.Mounts),
	}
	if s.WithNetHost {
		specOpts = append(specOpts, oci.WithHostNamespace(specs.NetworkNamespace), oci.WithHostHostsFile, oci.WithHostResolvconf, withHostCACertsFile)
	}
	if s.WithPrivileged {
		specOpts = append(specOpts, oci.WithPrivileged)
	}
	if s.WithAllDevicesAllowed {
		specOpts = append(specOpts, oci.WithAllDevicesAllowed)
	}
	if s.WithHostDevices {
		specOpts = append(specOpts, oci.WithHostDevices)
	}

	return client.NewContainer(
		ctx,
		s.ID,
		containerd.WithNewSpec(specOpts...),
	)
}

func (s *caaosService) start(ctx context.Context, client *containerd.Client) {
	if s.Delay != "" {
		if d, err := time.ParseDuration(s.Delay); err != nil {
			logger.Println("Error parsing delay:", err)
		} else {
			time.Sleep(d)
		}
	}

	container, err := s.getContainer(ctx, client)
	if err != nil {
		logger.Println("Error:", err)
		return
	}

	if err := runContainer(ctx, container); err != nil {
		logger.Println("Error:", err)
	}
}

func loadServices() []*caaosService {
	svcFileDir := "/etc/caaos"
	svcFiles, err := ioutil.ReadDir(svcFileDir)
	if err != nil {
		logger.Fatal(err)
	}

	var caaosServices []*caaosService
	for _, svcFile := range svcFiles {
		if svcFile.IsDir() {
			continue
		}
		data, err := ioutil.ReadFile(filepath.Join(svcFileDir, svcFile.Name()))
		if err != nil {
			logger.Println(err)
			continue
		}
		var svc caaosService
		if err := json.Unmarshal(data, &svc); err != nil {
			logger.Println(err)
			continue
		}

		caaosServices = append(caaosServices, &svc)
	}
	return caaosServices
}

func main() {
	logger.Println("Starting caaos...")
	ctx := namespaces.WithNamespace(context.Background(), "caaos")

	logger.Println("Reading caaos service files")
	svcs := loadServices()

	logger.Println("creating client")
	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		logger.Fatalln(err)
	}
	defer client.Close()

	go func() {
		for {
			select {
			case s := <-writerChan:
				os.Stdout.WriteString(s)
			}
		}
	}()

	logger.Println("Starting caaos services")
	for _, svc := range svcs {
		logger.Println("Starting", svc.ID)
		go svc.start(ctx, client)
	}

	for {
		logger.Println("Waiting for metadata...")
		md, err := watchMetadata(ctx)
		if err != nil {
			logger.Println("Error grabing metadata:", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if md.ContainerRef == "" {
			logger.Println("No container set, waiting...")
			continue
		}

		if err := runContainerFromImage(ctx, client, md.ContainerRef, md.ContainerSpec, md.OverwriteDefaults); err != nil {
			logger.Println("Error:", err)
			time.Sleep(5 * time.Second)
		}

		if md.StopOnExit {
			logger.Printf("Finished running %s, shutting down", md.ContainerRef)
			syscall.Sync()
			if err := syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF); err != nil {
				logger.Println("Error calling shutdown:", err)
			}
			select {}
		}

		logger.Printf("Finished running %s, waiting for next command...", md.ContainerRef)
	}
}
