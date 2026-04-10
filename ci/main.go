// A Dagger module for building and testing the Bloom cluster deployment tool
package main

import (
	"context"
	"dagger/bloom-ci/internal/dagger"
	"fmt"
)

type BloomCi struct{}

// Build the bloom binary from source
func (m *BloomCi) Build(
	ctx context.Context,
	// Source directory
	// +required
	source *dagger.Directory,
) *dagger.File {
	return dag.Container().
		From("golang:1.24-alpine").
		WithMountedDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"go", "mod", "download"}).
		WithExec([]string{"go", "build", "-o", "bloom", "-ldflags=-X 'github.com/silogen/cluster-bloom/cmd.Version=ci-build'"}).
		File("bloom")
}

// Run unit tests
func (m *BloomCi) Test(
	ctx context.Context,
	// Source directory
	// +required
	source *dagger.Directory,
) (string, error) {
	return dag.Container().
		From("golang:1.24-alpine").
		WithMountedDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"go", "mod", "download"}).
		WithExec([]string{"go", "test", "-v", "./pkg/..."}).
		Stdout(ctx)
}

// Validate bloom installation in a QEMU VM
// This runs the existing QEMU test script to validate the full installation flow
// Supports hardware acceleration on Linux (KVM), macOS (HVF), and Windows (WHPX)
func (m *BloomCi) ValidateInQemu(
	ctx context.Context,
	// Source directory
	// +required
	source *dagger.Directory,
	// VM configuration profile
	// +optional
	// +default="tests/qemu/profile_2_nvme.yaml"
	profile string,
	// Bloom configuration file
	// +optional
	// +default="tests/qemu/bloom.yaml"
	config string,
) (string, error) {
	// Build the bloom binary
	bloomBinary := m.Build(ctx, source)

	// Create Ubuntu container with QEMU and dependencies
	qemuContainer := dag.Container().
		From("ubuntu:24.04").
		WithExec([]string{"apt-get", "update"}).
		WithExec([]string{"apt-get", "install", "-y",
			"qemu-system-x86",
			"qemu-utils",
			"ovmf",
			"genisoimage",
			"openssh-client",
			"curl",
			"bash",
		}).
		WithMountedDirectory("/workspace", source).
		WithWorkdir("/workspace").
		WithFile("./bloom", bloomBinary).
		WithExec([]string{"chmod", "+x", "./bloom"})

	// Run the QEMU test
	vmName := "ci-test-vm"

	output, err := qemuContainer.
		WithExec([]string{
			"bash",
			"tests/qemu/manual-qemu-test.sh",
			vmName,
			profile,
			"./bloom",
			config,
		}).
		Stdout(ctx)

	if err != nil {
		return "", fmt.Errorf("QEMU validation failed: %w", err)
	}

	return output, nil
}

// Run the full CI pipeline: build, test, and optionally validate in QEMU
func (m *BloomCi) All(
	ctx context.Context,
	// Source directory
	// +required
	source *dagger.Directory,
	// Skip QEMU validation (requires KVM/nested virtualization)
	// +optional
	// +default=true
	skipQemu bool,
) (string, error) {
	// Run unit tests
	fmt.Println("Running unit tests...")
	testOutput, err := m.Test(ctx, source)
	if err != nil {
		return "", fmt.Errorf("unit tests failed: %w", err)
	}
	fmt.Println("✅ Unit tests passed")

	// Build the binary
	fmt.Println("Building bloom binary...")
	bloomBinary := m.Build(ctx, source)
	if bloomBinary == nil {
		return "", fmt.Errorf("build failed")
	}
	fmt.Println("✅ Build completed")

	// QEMU validation (optional, requires KVM)
	if !skipQemu {
		fmt.Println("Running QEMU validation...")
		qemuOutput, err := m.ValidateInQemu(ctx, source, "tests/qemu/profile_2_nvme.yaml", "tests/qemu/bloom.yaml")
		if err != nil {
			return "", fmt.Errorf("QEMU validation failed: %w", err)
		}
		fmt.Println("✅ QEMU validation passed")
		fmt.Println(qemuOutput)
	} else {
		fmt.Println("⏭️  Skipped QEMU validation (use --skip-qemu=false to enable)")
	}

	return fmt.Sprintf("✅ All CI checks passed\n\nTest output:\n%s", testOutput), nil
}

// Export the bloom binary to the dist/ directory
func (m *BloomCi) ExportBinary(
	ctx context.Context,
	// Source directory
	// +required
	source *dagger.Directory,
	// Output directory path on host
	// +default="../dist"
	outputPath string,
) (string, error) {
	bloomBinary := m.Build(ctx, source)

	_, err := bloomBinary.Export(ctx, outputPath+"/bloom")
	if err != nil {
		return "", fmt.Errorf("failed to export binary: %w", err)
	}

	return fmt.Sprintf("✅ Binary exported to %s/bloom", outputPath), nil
}
