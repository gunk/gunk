package downloader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gunk/gunk/log"
	"github.com/rogpeppe/go-internal/lockedfile"
)

type Paths struct {
	buildDir string
	binary   string
}

func getPaths(name, version string) (*Paths, func(error), error) {
	if version == "" {
		// require version. this is used only with version explicitly set.
		return nil, nil, fmt.Errorf("must provide protoc-gen-go version")
	}
	if !strings.HasPrefix(version, "v") {
		return nil, nil, fmt.Errorf("invalid version: %s", version)
	}
	// Get the OS-specific cache directory.
	cachePath, err := os.UserCacheDir()
	if err != nil {
		return nil, nil, err
	}
	if dir := os.Getenv("GUNK_CACHE_DIR"); dir != "" {
		// Allow overriding the cache dir entirely. Mainly for
		// the tests.
		cachePath = dir
	}
	cacheDir := filepath.Join(cachePath, "gunk")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, nil, err
	}
	pname := fmt.Sprintf("protoc-gen-%s-%s", name, version)
	var p Paths
	p.buildDir = filepath.Join(cacheDir, fmt.Sprintf("git-%s", pname))
	p.binary = filepath.Join(cacheDir, pname)
	lockPath := p.binary + ".lock"
	// Grab a lock separate from the destination file. The
	// destination file is a binary we'll want to execute, so using it
	// directly as the lock can lead to "text file busy" errors.
	unlock, err := lockedfile.MutexAt(lockPath).Lock()
	if err != nil {
		return nil, nil, err
	}
	cleanup := func(err error) {
		// if anything went wrong, remove binary, do not remove git
		// (just do remove and ignore errors)
		if err != nil {
			os.RemoveAll(p.binary)
		}
		unlock()
	}
	return &p, cleanup, nil
}

type Downloader interface {
	Name() string
	Download(version string, p Paths) (string, error)
}

var ds = []Downloader{
	Go{},
	GrpcJava{},
	GrpcEcosystem{Type: "grpc-gateway"},
	GrpcEcosystem{Type: "swagger"},
	GrpcEcosystem{Type: "openapiv2"},
	Swift{},
	GrpcSwift{},
	GrpcPython{},
	Ts{},
	GrpcGo{},
}

func Has(name string) bool {
	for _, d := range ds {
		if d.Name() == name {
			return true
		}
	}
	return false
}

func Download(name string, version string) (string, error) {
	for _, d := range ds {
		if d.Name() == name {
			s, err := download(d, version)
			if err != nil {
				name := fmt.Sprintf("protoc-gen-%s", d.Name())
				return "", fmt.Errorf("error downloading %s version %s: %w", name, version, err)
			}
			return s, nil
		}
	}
	return "", fmt.Errorf("unknown downloader %q", name)
}

func download(d Downloader, version string) (s string, err error) {
	p, cleanup, err := getPaths(d.Name(), version)
	if err != nil {
		return "", err
	}
	defer func() {
		// note - cannot do `defer cleanup(err)`, that might have wrong err
		cleanup(err)
	}()
	_, fErr := os.Stat(p.binary)
	if !os.IsNotExist(fErr) {
		if fErr != nil {
			return "", fErr
		}
		return p.binary, nil
	}
	// remove git clone dir here and not in cleanup,
	// so we can more easily debug
	// (ignore error)
	os.RemoveAll(p.buildDir)
	bin, err := d.Download(version, *p)
	if err != nil {
		return "", err
	}
	if bin != p.binary {
		// TODO windows?
		cpCmd := log.ExecCommand("ln",
			"-s",
			bin,
			p.binary)
		err = cpCmd.Run()
		if err != nil {
			return "", err
		}
	}
	return p.binary, nil
}
