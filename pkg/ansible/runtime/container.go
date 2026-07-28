package runtime

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

const (
	// ImageRef is the Ansible runtime image, pinned by digest for reproducible,
	// supply-chain-safe pulls (no floating :latest). The digest is the multi-arch
	// index, so crane still auto-selects the host platform.
	//
	// Corresponds to willhallonline/ansible:2.21.0-alpine-3.22 (Ansible 2.21.0,
	// Alpine 3.22) — the image :latest resolved to when this was pinned.
	// To bump: pick a new tag, resolve its index digest
	//   (docker manifest inspect willhallonline/ansible:<tag>), update both the
	//   digest below and ImageVersion, and re-run `just fetch-image` so the
	//   embedded (offline) build matches.
	ImageRef     = "willhallonline/ansible@sha256:9b819715663f18cfd0eb6a6fb1aedbc9d839781ffdd5f4faeff61b8c8a09ae26"
	ImageVersion = "2.21.0-alpine-3.22"
)

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

func ImageCached(rootfs string) bool {
	_, err := os.Stat(rootfs + "/usr")
	return err == nil
}

// ExtractEmbeddedRootfs unpacks a gzip-compressed, flattened rootfs tarball
// (produced by `just fetch-image` from the pinned ImageRef) into destPath. Used
// by the offline build (built with -tags embed_ansible_image) so bloom runs
// without any network pull of the Ansible runtime image.
func ExtractEmbeddedRootfs(destPath string, tarGz []byte) error {
	cmd := exec.Command("tar", "-xzf", "-", "-C", destPath)
	cmd.Stdin = bytes.NewReader(tarGz)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
