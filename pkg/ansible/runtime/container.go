package runtime

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

const (
	ImageRef = "willhallonline/ansible:2.19.14-alpine-3.24"

	// ansiblePlaybookInRootfs is where this image installs the playbook runner.
	// uv pip install --system puts the script in /usr/bin.
	ansiblePlaybookInRootfs = "usr/bin/ansible-playbook"

	pullAttempts     = 3
	pullStallTimeout = 60 * time.Second
)

// ansiblePlaybookPath is absolute so exec.Command does not LookPath a bare
// name. That lookup uses the host PATH and runs before Cmd.Env is applied.
func ansiblePlaybookPath() string {
	return "/" + ansiblePlaybookInRootfs
}

func imageRefStampPath(rootfs string) string {
	return rootfs + ".image-ref"
}

// registryTransport uses HTTP/1.1 only. Docker Hub's CDN can stop sending
// data on an HTTP/2 blob stream without closing it, and the default
// transport then waits forever.
func registryTransport() *http.Transport {
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		Protocols:             &protocols,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: pullStallTimeout,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// stallWatchdog cancels a pull when no data arrives for the timeout.
// The HTTP client has no read timeout of its own.
type stallWatchdog struct {
	timeout time.Duration
	timer   *time.Timer
	stalled atomic.Bool
}

func newStallWatchdog(timeout time.Duration, cancel context.CancelFunc) *stallWatchdog {
	w := &stallWatchdog{timeout: timeout}
	w.timer = time.AfterFunc(timeout, func() {
		w.stalled.Store(true)
		cancel()
	})
	return w
}

func (w *stallWatchdog) kick() { w.timer.Reset(w.timeout) }
func (w *stallWatchdog) stop() { w.timer.Stop() }

type watchedReader struct {
	r io.Reader
	w *stallWatchdog
}

func (wr watchedReader) Read(p []byte) (int, error) {
	n, err := wr.r.Read(p)
	if n > 0 {
		wr.w.kick()
	}
	return n, err
}

// PullAndExtractImage extracts imageRef into destPath. When an attempt fails,
// it empties destPath and starts again, up to pullAttempts times.
func PullAndExtractImage(imageRef, destPath string, verbose bool) error {
	var err error
	for attempt := 1; attempt <= pullAttempts; attempt++ {
		if attempt > 1 {
			fmt.Printf("  Download failed: %v\n  Retrying (attempt %d/%d)...\n", err, attempt, pullAttempts)
			if rerr := os.RemoveAll(destPath); rerr != nil {
				return fmt.Errorf("reset rootfs dir: %w", rerr)
			}
			if rerr := os.MkdirAll(destPath, 0755); rerr != nil {
				return fmt.Errorf("reset rootfs dir: %w", rerr)
			}
		}
		if err = pullAndExtractOnce(imageRef, destPath, verbose); err == nil {
			return nil
		}
	}
	return err
}

func pullAndExtractOnce(imageRef, destPath string, verbose bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watchdog := newStallWatchdog(pullStallTimeout, cancel)
	defer watchdog.stop()

	img, err := crane.Pull(imageRef, crane.WithContext(ctx), crane.WithTransport(registryTransport()))
	if err != nil {
		return fmt.Errorf("pulling image: %w", stallError(watchdog, err))
	}

	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("getting layers: %w", stallError(watchdog, err))
	}

	for i, layer := range layers {
		if verbose {
			fmt.Printf("  Extracting layer %d/%d...\n", i+1, len(layers))
		}
		watchdog.kick()
		if err := extractLayer(layer, destPath, watchdog); err != nil {
			return fmt.Errorf("extracting layer %d: %w", i+1, stallError(watchdog, err))
		}
	}
	return nil
}

func stallError(w *stallWatchdog, err error) error {
	if w.stalled.Load() {
		return fmt.Errorf("no data from registry for %s: %w", w.timeout, err)
	}
	return err
}

func extractLayer(layer v1.Layer, destPath string, watchdog *stallWatchdog) error {
	rc, err := layer.Uncompressed()
	if err != nil {
		return err
	}
	defer rc.Close()

	cmd := exec.Command("tar", "-xf", "-", "-C", destPath)
	cmd.Stdin = watchedReader{r: rc, w: watchdog}
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ImageCached reports whether rootfs is a complete extract of ImageRef.
// A directory that only contains /usr is not enough: the Alpine base layer
// creates /usr before the layer that installs ansible-playbook, and an
// interrupted extract must not be reused.
func ImageCached(rootfs string) bool {
	stamp, err := os.ReadFile(imageRefStampPath(rootfs))
	if err != nil || strings.TrimSpace(string(stamp)) != ImageRef {
		return false
	}
	info, err := os.Stat(filepath.Join(rootfs, ansiblePlaybookInRootfs))
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return false
	}
	return true
}

// MarkImageCached records that rootfs is a complete extract of ImageRef.
// Call it only after ansible-playbook is present in the extract.
func MarkImageCached(rootfs string) error {
	info, err := os.Stat(filepath.Join(rootfs, ansiblePlaybookInRootfs))
	if err != nil {
		return fmt.Errorf("ansible-playbook missing from runtime image: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return fmt.Errorf("ansible-playbook in runtime image is not an executable file")
	}
	if err := os.WriteFile(imageRefStampPath(rootfs), []byte(ImageRef+"\n"), 0644); err != nil {
		return fmt.Errorf("record ansible image ref: %w", err)
	}
	return nil
}

// DiscardRootfs removes a runtime extract and its image-ref stamp.
func DiscardRootfs(rootfs string) error {
	if err := os.RemoveAll(rootfs); err != nil {
		return fmt.Errorf("remove ansible rootfs: %w", err)
	}
	if err := os.Remove(imageRefStampPath(rootfs)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove ansible image ref: %w", err)
	}
	return nil
}
