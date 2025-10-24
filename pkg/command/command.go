/**
 * Copyright 2025 Advanced Micro Devices, Inc.  All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
**/

package command

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/silogen/cluster-bloom/pkg/dryrun"
	log "github.com/sirupsen/logrus"
)

// Run executes a command with real-time stderr logging
// Returns stdout as string and any error
// name: identifier for this call site (e.g., "StepName.Operation")
// runInDryRun: if true, command executes even in dry-run mode (for read-only operations)
func Run(name string, runInDryRun bool, command string, args ...string) (string, error) {
	if dryrun.IsDryRun() {
		// Always check for mocks first in dry-run mode
		if output, err, found := dryrun.GetMockValue(name); found {
			log.Infof("[DRY-RUN] %s: %s %s", name, command, strings.Join(args, " "))
			if err != nil {
				log.Infof("[DRY-RUN] %s: returning mock error: %v", name, err)
			} else {
				log.Debugf("[DRY-RUN] %s: returning mock output (%d bytes)", name, len(output))
			}
			return output, err
		}

		// If no mock and runInDryRun is false, return empty
		if !runInDryRun {
			log.Infof("[DRY-RUN] %s: %s %s", name, command, strings.Join(args, " "))
			return "", nil
		}

		// If no mock but runInDryRun is true, execute the real command
		// (This allows read-only operations to work without mocks if needed)
	}

	cmd := exec.Command(command, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			log.Debug(fmt.Sprintf("[%s %s] stderr: %s", command, strings.Join(args, " "), line))
		}
	}()

	stdoutBytes, err := io.ReadAll(stdout)
	if err != nil {
		return "", fmt.Errorf("failed to read stdout: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return string(stdoutBytes), fmt.Errorf("command failed: %w", err)
	}

	return string(stdoutBytes), nil
}

// CombinedOutput executes a command and returns combined stdout/stderr
// name: identifier for this call site (e.g., "StepName.Operation")
// runInDryRun: if true, command executes even in dry-run mode (for read-only operations)
func CombinedOutput(name string, runInDryRun bool, command string, args ...string) ([]byte, error) {
	if dryrun.IsDryRun() && !runInDryRun {
		log.Infof("[DRY-RUN] %s: %s %s", name, command, strings.Join(args, " "))

		// Check for mock return value
		if output, err, found := dryrun.GetMockValue(name); found {
			if err != nil {
				log.Infof("[DRY-RUN] %s: returning mock error: %v", name, err)
			} else {
				log.Debugf("[DRY-RUN] %s: returning mock output (%d bytes)", name, len(output))
			}
			return []byte(output), err
		}

		return []byte{}, nil
	}

	cmd := exec.Command(command, args...)
	return cmd.CombinedOutput()
}

// Output executes a command and returns stdout only
// name: identifier for this call site (e.g., "StepName.Operation")
// runInDryRun: if true, command executes even in dry-run mode (for read-only operations)
func Output(name string, runInDryRun bool, command string, args ...string) ([]byte, error) {
	if dryrun.IsDryRun() {
		// Always check for mocks first in dry-run mode
		if output, err, found := dryrun.GetMockValue(name); found {
			log.Infof("[DRY-RUN] %s: %s %s", name, command, strings.Join(args, " "))
			if err != nil {
				log.Infof("[DRY-RUN] %s: returning mock error: %v", name, err)
			} else {
				log.Debugf("[DRY-RUN] %s: returning mock output (%d bytes)", name, len(output))
			}
			return []byte(output), err
		}

		// If no mock and runInDryRun is false, return empty
		if !runInDryRun {
			log.Infof("[DRY-RUN] %s: %s %s", name, command, strings.Join(args, " "))
			return []byte{}, nil
		}

		// If no mock but runInDryRun is true, execute the real command
		// (This allows read-only operations to work without mocks if needed)
	}

	cmd := exec.Command(command, args...)
	return cmd.Output()
}

// SimpleRun executes a command and waits for it to complete
// Returns only error (no output capture)
// name: identifier for this call site (e.g., "StepName.Operation")
// runInDryRun: if true, command executes even in dry-run mode (for read-only operations)
func SimpleRun(name string, runInDryRun bool, command string, args ...string) error {
	if dryrun.IsDryRun() {
		// Always check for mocks first in dry-run mode
		if _, err, found := dryrun.GetMockValue(name); found {
			log.Infof("[DRY-RUN] %s: %s %s", name, command, strings.Join(args, " "))
			if err != nil {
				log.Infof("[DRY-RUN] %s: returning mock error: %v", name, err)
			}
			return err
		}

		// If no mock and runInDryRun is false, return nil
		if !runInDryRun {
			log.Infof("[DRY-RUN] %s: %s %s", name, command, strings.Join(args, " "))
			return nil
		}

		// If no mock but runInDryRun is true, execute the real command
		// (This allows read-only operations to work without mocks if needed)
	}

	cmd := exec.Command(command, args...)
	return cmd.Run()
}

// Cmd creates a *exec.Cmd that can be customized before execution
// This is for complex cases where you need to set Stdin, Stdout, Stderr, Dir, Env, etc.
// Returns nil in dry-run mode
func Cmd(command string, args ...string) *exec.Cmd {
	if dryrun.IsDryRun() {
		log.Infof("[DRY-RUN] CREATE_CMD: %s %s", command, strings.Join(args, " "))
		return nil
	}

	return exec.Command(command, args...)
}

// LookPath searches for an executable named file in the directories named by the PATH environment variable
// In dry-run mode, returns empty string and nil error to simulate command not found
func LookPath(file string) (string, error) {
	if dryrun.IsDryRun() {
		log.Infof("[DRY-RUN] LOOKPATH: %s", file)
		return "", nil
	}

	return exec.LookPath(file)
}
