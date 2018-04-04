package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"cloud.google.com/go/storage"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/google/shlex"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const (
	metadataURL  = "http://metadata.google.internal/computeMetadata/v1/instance/attributes"
	metadataHang = "/?recursive=true&alt=json&wait_for_change=true&timeout_sec=120&last_etag="
	defaultEtag  = "NONE"
	storageURL   = "storage.googleapis.com"

	bucket = `([a-z0-9][-_.a-z0-9]*)`
	object = `(.+)`
)

var (
	defaultTimeout = 130 * time.Second
	etag           = defaultEtag

	// Many of the Google Storage URLs are supported below.
	// It is preferred that customers specify their object using
	// its gs://<bucket>/<object> URL.
	bucketRegex = regexp.MustCompile(fmt.Sprintf(`^gs://%s/?$`, bucket))
	gsRegex     = regexp.MustCompile(fmt.Sprintf(`^gs://%s/%s$`, bucket, object))
	// Check for the Google Storage URLs:
	// http://<bucket>.storage.googleapis.com/<object>
	// https://<bucket>.storage.googleapis.com/<object>
	gsHTTPRegex1 = regexp.MustCompile(fmt.Sprintf(`^http[s]?://%s\.storage\.googleapis\.com/%s$`, bucket, object))
	// http://storage.cloud.google.com/<bucket>/<object>
	// https://storage.cloud.google.com/<bucket>/<object>
	gsHTTPRegex2 = regexp.MustCompile(fmt.Sprintf(`^http[s]?://storage\.cloud\.google\.com/%s/%s$`, bucket, object))
	// Check for the other possible Google Storage URLs:
	// http://storage.googleapis.com/<bucket>/<object>
	// https://storage.googleapis.com/<bucket>/<object>
	//
	// The following are deprecated but checked:
	// http://commondatastorage.googleapis.com/<bucket>/<object>
	// https://commondatastorage.googleapis.com/<bucket>/<object>
	gsHTTPRegex3 = regexp.MustCompile(fmt.Sprintf(`^http[s]?://(?:commondata)?storage\.googleapis\.com/%s/%s$`, bucket, object))

	logger = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds|log.Lshortfile)
)

type attributesJSON struct {
	CmdURL     string `json:"cmd-url"`
	CmdArgs    string `json:"cmd-args"`
	StopOnExit bool   `json:"stop-on-exit,string"`
}

func downloadGSURL(ctx context.Context, bucket, object string, file *os.File) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}
	defer client.Close()

	bkt := client.Bucket(bucket)
	obj := bkt.Object(object)
	r, err := obj.NewReader(ctx)
	if err != nil {
		return fmt.Errorf("error reading object %q: %v", object, err)
	}
	defer r.Close()

	_, err = io.Copy(file, r)
	return err
}

func downloadCmd(ctx context.Context, url string) (string, error) {
	out := filepath.Join("/usr", path.Base(url))
	file, err := os.OpenFile(out, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return "", err
	}
	defer file.Close()

	bucket, object := findMatch(url)
	if bucket != "" && object != "" {
		// Retry up to 3 times, only wait 1 second between retries.
		for i := 1; ; i++ {
			logger.Printf("Downloading from GCS, bucket: %q, object: %q", bucket, object)
			err = downloadGSURL(ctx, bucket, object, file)
			if err == nil {
				return out, nil
			}
			if err != nil && i > 3 {
				logger.Println("Failed to download GCS path:", err)
				break
			}
			logger.Print("Failed to download GCS path, retrying...")
			time.Sleep(1 * time.Second)
		}
		logger.Print("Trying unauthenticated download")
		return out, downloadURL(fmt.Sprintf("https://%s/%s/%s", storageURL, bucket, object), file)
	}

	// Fall back to an HTTP GET of the URL.
	return out, downloadURL(url, file)
}

func downloadURL(url string, file *os.File) error {
	logger.Printf("Downloading from URL: %q", url)
	// Retry up to 3 times, only wait 1 second between retries.
	var res *http.Response
	var err error
	for i := 1; ; i++ {
		res, err = http.Get(url)
		if err != nil && i > 3 {
			return err
		}
		if err == nil {
			break
		}
		logger.Print("Failed to download URL, retrying...")
		time.Sleep(1 * time.Second)
	}

	defer res.Body.Close()
	_, err = io.Copy(file, res.Body)
	return err
}

func findMatch(path string) (string, string) {
	for _, re := range []*regexp.Regexp{gsRegex, gsHTTPRegex1, gsHTTPRegex2, gsHTTPRegex3} {
		match := re.FindStringSubmatch(path)
		if len(match) == 3 {
			return match[1], match[2]
		}
	}
	return "", ""
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

func main() {
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err != nil {
		logger.Fatalln("Error from Sysinfo:", err)
	}

	logger.Printf("[%d] Starting c-nix init...", info.Uptime)

	logger.Println("mounting proc")
	if err := syscall.Mount("proc", "/proc", "proc", 0, "ro"); err != nil {
		logger.Fatalln(err)
	}

	logger.Println("starting containerd")
	cmd := exec.Command("/bin/containerd")
	cmd.Env = []string{"PATH=/bin"}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		logger.Fatalln(err)
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			logger.Fatalln(err)
		}
	}()

	logger.Println("creating client")
	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		logger.Fatalln(err)
	}
	defer client.Close()

	ctx := namespaces.WithNamespace(context.Background(), "caaos")

	id := "docker.io/library/busybox:latest"

	logger.Println("pulling image")
	img, err := client.Pull(ctx, id, containerd.WithPullUnpack)
	if err != nil {
		logger.Println(err)
		time.Sleep(5 * time.Second)
		return
	}

	rnd := fmt.Sprintf("%d", time.Now().Unix())

	logger.Println("creating container")
	spec := containerd.WithNewSpec(
		oci.WithImageConfig(img),
		oci.WithHostNamespace(specs.NetworkNamespace),
		oci.WithHostHostsFile,
		oci.WithHostResolvconf,
	)

	container, err := client.NewContainer(
		ctx,
		rnd,
		containerd.WithImage(img),
		containerd.WithNewSnapshot(rnd, img),
		spec,
	)
	if err != nil {
		logger.Println(err)
		time.Sleep(5 * time.Second)
		return
	}
	defer container.Delete(ctx, containerd.WithSnapshotCleanup)

	// create a new task
	logger.Println("creating task")
	task, err := container.NewTask(ctx, cio.Stdio)
	if err != nil {
		logger.Println(err)
		time.Sleep(5 * time.Second)
		return
	}

	// the task is now running and has a pid that can be use to setup networking
	// or other runtime settings outside of containerd
	pid := task.Pid()

	fmt.Println(pid)

	// Setup wait channel
	statusC, err := task.Wait(ctx)
	if err != nil {
		logger.Println(err)
		time.Sleep(5 * time.Second)
		return
	}

	// start the redis-server process inside the container
	logger.Println("running task")
	if err := task.Start(ctx); err != nil {
		logger.Println(err)
		time.Sleep(5 * time.Second)
		return
	}

	// wait for the task to exit and get the exit status
	logger.Println("waiting...")
	status := <-statusC
	code, _, err := status.Result()
	if err != nil {
		logger.Println(err)
		time.Sleep(5 * time.Second)
		return
	}

	fmt.Println(code)

	logger.Println("deleting task")
	if _, err := task.Delete(ctx); err != nil {
		logger.Println(err)
	}

	// kill the process and get the exit status
	//if err := task.Kill(ctx, syscall.SIGTERM); err != nil {
	//	logger.Println(err)
	//}

	return

	for {
		md, err := watchMetadata(ctx)
		if err != nil {
			logger.Println("Error grabing metadata:", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if md.CmdURL == "" {
			logger.Println("Waiting for command...")
			continue
		}

		args, err := shlex.Split(md.CmdArgs)
		if err != nil {
			logger.Println("Error parsing arguments:", err)
			continue
		}

		cmd, err := downloadCmd(ctx, md.CmdURL)
		if err != nil {
			logger.Println("Error downloading command:", err)
			continue
		}

		if err := runCmd(ctx, cmd, args); err != nil {
			logger.Println("Error running command:", err)
			continue
		}
		if md.StopOnExit {
			logger.Printf("Finished running %s, shutting down", cmd)
			syscall.Sync()
			if err := syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF); err != nil {
				logger.Println("Error calling shutdown:", err)
			}
			time.Sleep(5 * time.Second)
			return
		}
		logger.Printf("Finished running %s, waiting for next command...\n", cmd)
	}
}
