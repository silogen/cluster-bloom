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

package pkg

import (
	"testing"
	"time"
)

func TestDemoCheckUbuntuStep(t *testing.T) {
	step := DemoCheckUbuntuStep

	if step.Name != "Demo Check Ubuntu" {
		t.Errorf("Expected Name 'Demo Check Ubuntu', got '%s'", step.Name)
	}
	if step.Description != "Demo step to simulate Ubuntu version check" {
		t.Errorf("Expected specific description, got '%s'", step.Description)
	}
	if step.Action == nil {
		t.Errorf("Expected non-nil Action")
	}

	start := time.Now()
	result := step.Action()
	duration := time.Since(start)

	if result.Error != nil {
		t.Errorf("Expected no error, got: %v", result.Error)
	}
	if duration < 1*time.Second {
		t.Errorf("Expected step to take at least 1 second, took %v", duration)
	}
}

func TestDemoPackagesStep(t *testing.T) {
	step := DemoPackagesStep

	if step.Name != "Demo Install Packages" {
		t.Errorf("Expected Name 'Demo Install Packages', got '%s'", step.Name)
	}
	if step.Description != "Demo step to simulate package installation" {
		t.Errorf("Expected specific description, got '%s'", step.Description)
	}
	if step.Action == nil {
		t.Errorf("Expected non-nil Action")
	}

	start := time.Now()
	result := step.Action()
	duration := time.Since(start)

	if result.Error != nil {
		t.Errorf("Expected no error, got: %v", result.Error)
	}
	if duration < 2*time.Second {
		t.Errorf("Expected step to take at least 2 seconds, took %v", duration)
	}
}

func TestDemoFirewallStep(t *testing.T) {
	step := DemoFirewallStep

	if step.Name != "Demo Configure Firewall" {
		t.Errorf("Expected Name 'Demo Configure Firewall', got '%s'", step.Name)
	}
	if step.Description != "Demo step to simulate firewall configuration" {
		t.Errorf("Expected specific description, got '%s'", step.Description)
	}
	if step.Action == nil {
		t.Errorf("Expected non-nil Action")
	}

	result := step.Action()
	if result.Error != nil {
		t.Errorf("Expected no error, got: %v", result.Error)
	}
}

func TestDemoMinioStep(t *testing.T) {
	step := DemoMinioStep

	if step.Name != "Demo MinIO Setup" {
		t.Errorf("Expected Name 'Demo MinIO Setup', got '%s'", step.Name)
	}
	if step.Description != "Demo step to simulate MinIO installation" {
		t.Errorf("Expected specific description, got '%s'", step.Description)
	}
	if step.Action == nil {
		t.Errorf("Expected non-nil Action")
	}

	start := time.Now()
	result := step.Action()
	duration := time.Since(start)

	if result.Error != nil {
		t.Errorf("Expected no error, got: %v", result.Error)
	}
	if duration < 1*time.Second {
		t.Errorf("Expected step to take at least 1 second, took %v", duration)
	}
}

func TestDemoDashboardStep(t *testing.T) {
	step := DemoDashboardStep

	if step.Name != "Demo Dashboard" {
		t.Errorf("Expected Name 'Demo Dashboard', got '%s'", step.Name)
	}
	if step.Description != "Demo step to simulate dashboard setup" {
		t.Errorf("Expected specific description, got '%s'", step.Description)
	}
	if step.Action == nil {
		t.Errorf("Expected non-nil Action")
	}

	result := step.Action()
	if result.Error != nil {
		t.Errorf("Expected no error, got: %v", result.Error)
	}
}

func TestAllDemoSteps(t *testing.T) {
	steps := []Step{
		DemoCheckUbuntuStep,
		DemoPackagesStep,
		DemoFirewallStep,
		DemoMinioStep,
		DemoDashboardStep,
	}

	for _, step := range steps {
		t.Run(step.Name, func(t *testing.T) {
			result := step.Action()
			if result.Error != nil {
				t.Errorf("Demo step %s returned error: %v", step.Name, result.Error)
			}
		})
	}
}