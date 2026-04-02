package runtime

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

const (
	ImageRef = "willhallonline/ansible:latest"
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
