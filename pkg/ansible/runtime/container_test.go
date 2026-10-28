package runtime

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImageCachedRequiresStampAndExecutable(t *testing.T) {
	rootfs := filepath.Join(t.TempDir(), "rootfs")
	if err := os.MkdirAll(rootfs, 0755); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(rootfs, ansiblePlaybookInRootfs)

	if ImageCached(rootfs) {
		t.Fatal("ImageCached() = true for an empty rootfs")
	}

	if err := os.MkdirAll(filepath.Dir(binary), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("#!/usr/bin/python3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if ImageCached(rootfs) {
		t.Fatal("ImageCached() = true without an image-ref stamp")
	}

	if err := os.WriteFile(imageRefStampPath(rootfs), []byte(ImageRef+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if ImageCached(rootfs) {
		t.Fatal("ImageCached() = true when ansible-playbook is not executable")
	}

	if err := os.Chmod(binary, 0755); err != nil {
		t.Fatal(err)
	}
	if !ImageCached(rootfs) {
		t.Fatal("ImageCached() = false for a stamped rootfs with an executable ansible-playbook")
	}

	if err := os.WriteFile(imageRefStampPath(rootfs), []byte("willhallonline/ansible:other\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if ImageCached(rootfs) {
		t.Fatal("ImageCached() = true when the image-ref stamp does not match ImageRef")
	}
}

func TestMarkImageCachedRejectsMissingBinary(t *testing.T) {
	rootfs := filepath.Join(t.TempDir(), "rootfs")
	if err := os.MkdirAll(rootfs, 0755); err != nil {
		t.Fatal(err)
	}
	if err := MarkImageCached(rootfs); err == nil {
		t.Fatal("MarkImageCached() error = nil, want missing ansible-playbook")
	}
	if ImageCached(rootfs) {
		t.Fatal("ImageCached() = true after MarkImageCached failed")
	}
}

func TestMarkImageCachedThenDiscard(t *testing.T) {
	rootfs := filepath.Join(t.TempDir(), "rootfs")
	if err := os.MkdirAll(rootfs, 0755); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(rootfs, ansiblePlaybookInRootfs)
	if err := os.MkdirAll(filepath.Dir(binary), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("#!/usr/bin/python3\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := MarkImageCached(rootfs); err != nil {
		t.Fatal(err)
	}
	if !ImageCached(rootfs) {
		t.Fatal("ImageCached() = false after MarkImageCached")
	}
	if err := DiscardRootfs(rootfs); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(rootfs); !os.IsNotExist(err) {
		t.Fatalf("rootfs stat err = %v, want not exist", err)
	}
	if _, err := os.Stat(imageRefStampPath(rootfs)); !os.IsNotExist(err) {
		t.Fatalf("stamp stat err = %v, want not exist", err)
	}
}

func TestRegistryTransportUsesHTTP1AgainstHTTP2Server(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Proto)
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()

	tr := registryTransport()
	roots := x509.NewCertPool()
	roots.AddCert(srv.Certificate())
	tr.TLSClientConfig = &tls.Config{RootCAs: roots}
	resp, err := (&http.Client{Transport: tr}).Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.ProtoMajor != 1 || string(body) != "HTTP/1.1" {
		t.Fatalf("negotiated %s (server saw %q), want HTTP/1.1", resp.Proto, body)
	}
}

func TestStallWatchdogCancelsWithoutData(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := newStallWatchdog(50*time.Millisecond, cancel)
	defer w.stop()

	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("watchdog did not cancel the context")
	}
	if !w.stalled.Load() {
		t.Fatal("stalled = false after the watchdog fired")
	}
}

func TestStallWatchdogKeepsActivePullAlive(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := newStallWatchdog(200*time.Millisecond, cancel)
	defer w.stop()

	r := watchedReader{r: strings.NewReader(strings.Repeat("x", 10)), w: w}
	buf := make([]byte, 1)
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		if _, err := r.Read(buf); err != nil {
			t.Fatal(err)
		}
	}
	if ctx.Err() != nil || w.stalled.Load() {
		t.Fatal("watchdog fired while data was arriving")
	}
}
