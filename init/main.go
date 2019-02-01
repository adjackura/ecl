package main

import (
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/unix"
)

var (
	logger *log.Logger
)

func init() {
	kmsg, err := os.OpenFile("/dev/kmsg", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error, falling back to stdout:", err)
		kmsg = os.Stdout
	}
	logger = log.New(kmsg, "[init]: ", log.Lmicroseconds)
}

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

func start(path string, args ...string) error {
	cmd := exec.Command(path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			logger.Println(err)
		}
		start(path, args...)
	}()
	return nil
}

func main() {
	logger.Println("Starting ecl...")

	logger.Println("Mounting all the things")
	mounts()

	logger.Println("Starting container...")
	//if err := start("/bin/runc", "run", "-b", "/container", "container"); err != nil {
	//	logger.Fatalln(err)
	//}
	if err := start("/sbin/container-init"); err != nil {
		logger.Fatalln(err)
	}

	select {}
}
