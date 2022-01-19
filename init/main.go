package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

var (
	writerChan = make(chan string, 10)
	logger     = log.New(&consoleWriter{name: "init"}, "", log.LstdFlags|log.Lmicroseconds)
	start      = time.Now()
)

const (
	nodev    = unix.MS_NODEV
	noexec   = unix.MS_NOEXEC
	nosuid   = unix.MS_NOSUID
	readonly = unix.MS_RDONLY
	rec      = unix.MS_REC
	relatime = unix.MS_RELATIME
	remount  = unix.MS_REMOUNT
	shared   = unix.MS_SHARED
	bind     = unix.MS_BIND
)

type consoleWriter struct {
	name string
}

func (w *consoleWriter) Write(b []byte) (int, error) {
	t := time.Since(start).Seconds()
	var msg string
	for _, b := range bytes.Split(bytes.TrimRight(b, "\n"), []byte("\n")) {
		msg += fmt.Sprintf("[ %f ] [%s] %s\n", t, w.name, b)
	}
	writerChan <- msg
	return len(b), nil
}

func setupLogging() {
	// mount proc filesystem
	mount("proc", "/proc", "proc", nodev|nosuid|noexec|relatime, "")

	f, err := os.Open("/proc/uptime")
	if err == nil {
		defer f.Close()
		d, err := io.ReadAll(f)
		if err == nil {
			uptime := bytes.Split(d, []byte(" "))[0]
			u, err := strconv.ParseFloat(string(uptime), 32)
			if err == nil {
				start = start.Add(-time.Duration(int(u*1000)) * time.Millisecond)
			}
		}
	}

	go func() {
		for {
			select {
			case s := <-writerChan:
				os.Stdout.WriteString(s)
			}
		}
	}()
}

func mount(source string, target string, fstype string, flags uintptr, data string) {
	if err := unix.Mount(source, target, fstype, flags, data); err != nil {
		logger.Printf("error mounting %s to %s: %v", source, target, err)
	}
}

func mkdir(path string, perm os.FileMode) {
	if err := os.MkdirAll(path, perm); err != nil {
		logger.Printf("error making directory %s: %v", path, err)
	}
}

func symlink(oldpath string, newpath string) {
	if err := unix.Symlink(oldpath, newpath); err != nil {
		logger.Printf("error making symlink %s: %v", newpath, err)
	}
}

func write(path string, value string) {
	err := ioutil.WriteFile(path, []byte(value), 0600)
	if err != nil {
		logger.Printf("cannot write to %s: %v", path, err)
	}
}

func mounts() {
	mount("/dev/sda3", "/mnt", "ext4", nodev|nosuid|relatime, "")
	mount("/mnt/var", "/var", "", bind, "")
	mount("/mnt/opt", "/opt", "", bind, "")
	if err := unix.Unmount("/mnt", 0); err != nil {
		logger.Printf("error unmounting %s: %v", "/mnt", err)
	}

	// mount tmpfs for /tmp and /run
	mount("tmpfs", "/run", "tmpfs", nodev|nosuid|noexec|relatime, "size=10%,mode=755")
	mount("tmpfs", "/tmp", "tmpfs", nodev|nosuid|noexec|relatime, "size=10%,mode=1777")

	// add standard directories in /var
	mkdir("/var/cache", 0755)
	mkdir("/var/empty", 0555)
	mkdir("/var/lib", 0755)
	mkdir("/var/local", 0755)
	mkdir("/var/lock", 0755)
	mkdir("/var/log", 0755)
	mkdir("/var/opt", 0755)
	mkdir("/var/spool", 0755)
	mkdir("/var/tmp", 01777)
	symlink("/run", "/var/run")

	// make standard symlinks
	symlink("/proc/self/fd", "/dev/fd")
	symlink("/proc/self/fd/0", "/dev/stdin")
	symlink("/proc/self/fd/1", "/dev/stdout")
	symlink("/proc/self/fd/2", "/dev/stderr")
	symlink("/proc/kcore", "/dev/kcore")

	// sysfs
	mount("sysfs", "/sys", "sysfs", noexec|nosuid|nodev, "")

	mount("cgroup2", "/sys/fs/cgroup", "cgroup2", noexec|nosuid|nodev, "")
}

type systemService struct {
	name, path string
	args       []string
}

func (s *systemService) start() error {
	cmd := exec.Command(s.path, s.args...)
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/bin:/usr/local/sbin:/opt/bin"}
	w := &consoleWriter{name: s.name}
	cmd.Stdout = w
	cmd.Stderr = w

	if err := cmd.Start(); err != nil {
		logger.Fatalln(err)
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			logger.Println(err)
		}
		s.start()
	}()
	return nil
}

var systemServices = map[string]*systemService{}

func main() {
	os.Stdout.WriteString("Starting AgileOS...\n")
	setupLogging()

	logger.Println("Mounting all the things")
	mounts()

	logger.Println("Running ACPI listener")
	go func() {
		if err := runACPIListener(); err != nil {
			logger.Println("Error running acpi listener:", err)
			return
		}
	}()

	logger.Println("Reading core service files")
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
		var svc systemService
		for scanner.Scan() {
			entry := strings.SplitN(scanner.Text(), "=", 2)
			if len(entry) != 2 {
				continue
			}
			switch entry[0] {
			case "NAME":
				svc.name = strings.Trim(entry[1], `"`)
			case "PATH":
				svc.path = strings.Trim(entry[1], `"`)
			case "ARGS":
				svc.args = strings.Split(strings.Replace(entry[1], " ", "", -1), ",")
			}
		}

		systemServices[svcFile.Name()] = &svc
	}

	var keys []string
	for k := range systemServices {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	logger.Println("Starting services")
	for _, k := range keys {
		logger.Println("Starting", systemServices[k].name)
		systemServices[k].start()
	}

	select {}
}
