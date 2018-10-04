package main

import (
	"bufio"
	"encoding/csv"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

var (
	logger = log.New(os.Stdout, "[init]: ", log.LstdFlags|log.Lmicroseconds|log.Lshortfile)
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
)

func mount(source string, target string, fstype string, flags uintptr, data string) {
	err := unix.Mount(source, target, fstype, flags, data)
	if err != nil {
		logger.Printf("error mounting %s to %s: %v", source, target, err)
	}
}

func mkdir(path string, perm os.FileMode) {
	err := os.MkdirAll(path, perm)
	if err != nil {
		logger.Printf("error making directory %s: %v", path, err)
	}
}

func symlink(oldpath string, newpath string) {
	err := unix.Symlink(oldpath, newpath)
	if err != nil {
		logger.Printf("error making symlink %s: %v", newpath, err)
	}
}

func cgroupList() []string {
	list := []string{}
	f, err := os.Open("/proc/cgroups")
	if err != nil {
		logger.Printf("cannot open /proc/cgroups: %v", err)
		return list
	}
	defer f.Close()
	reader := csv.NewReader(f)
	// tab delimited
	reader.Comma = '\t'
	// four fields
	reader.FieldsPerRecord = 4
	cgroups, err := reader.ReadAll()
	if err != nil {
		logger.Printf("cannot parse /proc/cgroups: %v", err)
		return list
	}
	for _, cg := range cgroups {
		// see if enabled
		if cg[3] == "1" {
			list = append(list, cg[0])
		}
	}
	return list
}

func write(path string, value string) {
	err := ioutil.WriteFile(path, []byte(value), 0600)
	if err != nil {
		logger.Printf("cannot write to %s: %v", path, err)
	}
}

func mounts() {
	mkdir("/mnt", 0755)
	mkdir("/root", 0700)
	mkdir("/cntr", 0755)

	// mount proc filesystem
	mkdir("/proc", 0755)
	mount("proc", "/proc", "proc", nodev|nosuid|noexec|relatime, "")

	// mount tmpfs for /tmp and /run
	mkdir("/run", 0755)
	mount("tmpfs", "/run", "tmpfs", nodev|nosuid|noexec|relatime, "size=10%,mode=755")
	mkdir("/tmp", 1777)
	mount("tmpfs", "/tmp", "tmpfs", nodev|nosuid|noexec|relatime, "size=10%,mode=1777")

	// mount tmpfs for /var. This may be overmounted with a persistent filesystem later
	mkdir("/var", 0755)
	mount("tmpfs", "/var", "tmpfs", nodev|nosuid|noexec|relatime, "size=50%,mode=755")
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
	mkdir("/sys", 0755)
	mount("sysfs", "/sys", "sysfs", noexec|nosuid|nodev, "")

	// mount cgroup root tmpfs
	mount("cgroup_root", "/sys/fs/cgroup", "tmpfs", nodev|noexec|nosuid, "mode=755,size=10m")
	// mount cgroups filesystems for all enabled cgroups
	for _, cg := range cgroupList() {
		path := filepath.Join("/sys/fs/cgroup", cg)
		mkdir(path, 0555)
		mount(cg, path, "cgroup", noexec|nosuid|nodev, cg)
	}

	// use hierarchy for memory
	write("/sys/fs/cgroup/memory/memory.use_hierarchy", "1")
}

type systemService struct {
	name, desc, path string
	args             []string
	running          bool
	mx               sync.RWMutex
}

func (s *systemService) isRunning() bool {
	s.mx.RLock()
	defer s.mx.RUnlock()
	return s.running
}

func (s *systemService) start() error {
	cmd := exec.Command(s.path, s.args...)
	cmd.Env = []string{"PATH=/bin"}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
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
	logger.Println("Starting ecl...")

	logger.Println("Mounting all the things")
	mounts()

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
		var svc systemService
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
