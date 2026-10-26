package runtime

import (
	"os"
	"path/filepath"
	"testing"
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
