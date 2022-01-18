package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
)

const (
	metadataURL  = "http://metadata.google.internal/computeMetadata/v1/instance/attributes"
	metadataHang = "/?recursive=true&alt=json&wait_for_change=true&timeout_sec=120&last_etag="
	defaultEtag  = "NONE"
)

var (
	defaultTimeout = 130 * time.Second
	etag           = defaultEtag

	logger = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds|log.Lshortfile)
)

type attributesJSON struct {
	ContainerRef      string `json:"container-ref"`
	ContainerSpec     string `json:"container-spec"`
	OverwriteDefaults bool   `json:"overwrite-defaults,string"`
	StopOnExit        bool   `json:"stop-on-exit,string"`
}

func runCmd(ctx context.Context, path string, args []string) error {
	logger.Printf("Running %q with args %q", path, args)

	c := exec.Command(path, args...)

	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	defer pr.Close()

	c.Stdout = pw
	c.Stderr = pw

	if err := c.Start(); err != nil {
		return err
	}
	pw.Close()

	in := bufio.NewScanner(pr)
	for in.Scan() {
		logger.Printf("%s: %s", filepath.Base(path), in.Text())
	}

	return c.Wait()
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

func runContainer(ctx context.Context, client *containerd.Client, ref string, spec string, clear bool) error {
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

	s, _ := container.Spec(ctx)
	d, _ := json.Marshal(s)
	fmt.Println("Container spec:", string(d))

	// create a new task
	logger.Println("Creating task")
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return err
	}

	// Setup wait channel
	statusC, err := task.Wait(ctx)
	if err != nil {
		return err
	}

	// start the task
	logger.Println("Running task")
	if err := task.Start(ctx); err != nil {
		return err
	}

	// wait for the task to exit and get the exit status
	logger.Println("Waiting...")
	status := <-statusC
	code, _, err := status.Result()
	if err != nil {
		return err
	}

	logger.Println("Return code:", code)

	logger.Println("Deleting task")
	if _, err := task.Delete(ctx); err != nil {
		logger.Println(err)
	}

	// kill the process and get the exit status
	//if err := task.Kill(ctx, syscall.SIGTERM); err != nil {
	//	logger.Println(err)
	//}

	return nil
}

type caaosService struct {
	name, desc, path string
	args             []string
	delay            string
}

var caaosServices = map[string]*caaosService{}

func readConfigs() {
	logger.Println("Reading service files")
	svcFileDir := "/etc/init"
	svcFiles, err := ioutil.ReadDir(svcFileDir)
	if err != nil {
		logger.Fatal(err)
	}

	for _, svcFile := range svcFiles {
		if svcFile.IsDir() {
			continue
		}
		file := filepath.Join(svcFileDir, svcFile.Name())
		f, err := os.Open(file)
		if err != nil {
			logger.Printf("Error opening service file %s: %v", file, err)
			continue
		}
		scanner := bufio.NewScanner(f)
		var svc caaosService
		for scanner.Scan() {
			entry := strings.SplitN(scanner.Text(), "=", 2)
			if len(entry) != 2 {
				continue
			}
			switch entry[0] {
			case "NAME":
				svc.name = strings.Trim(entry[1], `"`)
			case "DESCRIPTION":
				svc.desc = strings.Trim(entry[1], `"`)
			case "PATH":
				svc.path = strings.Trim(entry[1], `"`)
			case "ARGS":
				svc.args = strings.Split(strings.Replace(entry[1], " ", "", -1), ",")
			case "DELAY":
				svc.delay = strings.Trim(entry[1], `"`)
			}
		}

		caaosServices[svcFile.Name()] = &svc
	}
}

func main() {
	logger.Println("Starting caaos...")

	logger.Println("creating client")
	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		logger.Fatalln(err)
	}
	defer client.Close()

	ctx := namespaces.WithNamespace(context.Background(), "caaos")

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

		if err := runContainer(ctx, client, md.ContainerRef, md.ContainerSpec, md.OverwriteDefaults); err != nil {
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
