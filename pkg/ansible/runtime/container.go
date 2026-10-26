package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

const (
	ImageRef = "willhallonline/ansible:2.19.14-alpine-3.24"

	// ansiblePlaybookInRootfs is where this image installs the playbook runner.
	// uv pip install --system puts the script in /usr/bin.
	ansiblePlaybookInRootfs = "usr/bin/ansible-playbook"
)

// ansiblePlaybookPath is absolute so exec.Command does not LookPath a bare
// name. That lookup uses the host PATH and runs before Cmd.Env is applied.
func ansiblePlaybookPath() string {
	return "/" + ansiblePlaybookInRootfs
}

func imageRefStampPath(rootfs string) string {
	return rootfs + ".image-ref"
}

func PullAndExtractImage(imageRef, destPath string, verbose bool) error {
	img, err := crane.Pull(imageRef)
	if err != nil {
		return fmt.Errorf("pulling image: %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("getting layers: %w", err)
	}

	for i, layer := range layers {
		if verbose {
			fmt.Printf("  Extracting layer %d/%d...\n", i+1, len(layers))
		}
		if err := extractLayer(layer, destPath); err != nil {
			return fmt.Errorf("extracting layer %d: %w", i+1, err)
		}
	}
	return nil
}

func extractLayer(layer v1.Layer, destPath string) error {
	rc, err := layer.Uncompressed()
	if err != nil {
		return err
	}
	defer rc.Close()

	cmd := exec.Command("tar", "-xf", "-", "-C", destPath)
	cmd.Stdin = rc
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
