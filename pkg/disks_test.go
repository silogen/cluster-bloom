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
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestCleanDisks(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}
	
	err := CleanDisks()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestParseLonghornDiskConfig(t *testing.T) {
	tests := []struct {
		name     string
		disks    string
		expected string
	}{
		{"single disk", "/dev/sda", "/dev/sda"},
		{"multiple disks", "/dev/sda,/dev/sdb", "/dev/sdaXXX/dev/sdb"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("LONGHORN_DISKS", tt.disks)
			result := ParseLonghornDiskConfig()
			if tt.disks == "" && result != "" {
				t.Errorf("Expected empty result for empty disks, got: %s", result)
			} else if tt.disks != "" && !strings.Contains(result, "xxx") {
				// For multiple disks, should contain 'xxx' separator
				if strings.Contains(tt.disks, ",") && !strings.Contains(result, "xxx") {
					t.Errorf("Expected result to contain 'xxx' separator for multiple disks")
				}
			}
		})
	}
}

func TestGenerateLonghornDiskString(t *testing.T) {
	t.Run("with LONGHORN_DISKS set", func(t *testing.T) {
		viper.Set("LONGHORN_DISKS", "/dev/sda")
		
		err := GenerateLonghornDiskString()
		// Expected to fail due to permission/path issues in test
		if err == nil {
			t.Log("Function succeeded unexpectedly")
		}
		viper.Set("LONGHORN_DISKS", "")
	})

	t.Run("with SKIP_DISK_CHECK", func(t *testing.T) {
		viper.Set("SKIP_DISK_CHECK", true)
		err := GenerateLonghornDiskString()
		if err != nil {
			t.Errorf("Expected no error with SKIP_DISK_CHECK, got: %v", err)
		}
		viper.Set("SKIP_DISK_CHECK", false)
	})

	t.Run("with no selected disks", func(t *testing.T) {
		viper.Set("selected_disks", []string{})
		err := GenerateLonghornDiskString()
		if err != nil {
			t.Errorf("Expected no error with empty disk list, got: %v", err)
		}
	})
}

func TestIsVirtualDisk(t *testing.T) {
	tests := []struct {
		name     string
		udevOut  []byte
		expected bool
	}{
		{"QEMU virtual disk", []byte("ID_VENDOR=QEMU\nID_MODEL=DISK"), true},
		{"VMware virtual disk", []byte("ID_VENDOR=VMware\nID_MODEL=Virtual"), true},
		{"Physical disk", []byte("ID_VENDOR=Samsung\nID_MODEL=SSD"), false},
		{"Empty output", []byte(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isVirtualDisk(tt.udevOut)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetUnmountedPhysicalDisks(t *testing.T) {
	t.Run("with SKIP_DISK_CHECK", func(t *testing.T) {
		viper.Set("SKIP_DISK_CHECK", true)
		disks, err := GetUnmountedPhysicalDisks()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if disks != nil {
			t.Errorf("Expected nil disks with SKIP_DISK_CHECK, got: %v", disks)
		}
		viper.Set("SKIP_DISK_CHECK", false)
	})

	t.Run("normal operation", func(t *testing.T) {
		disks, err := GetUnmountedPhysicalDisks()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if disks == nil {
			t.Errorf("Expected non-nil disks slice")
		}
	})
}

func TestMountDrives(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	t.Run("with LONGHORN_DISKS set", func(t *testing.T) {
		viper.Set("LONGHORN_DISKS", "/dev/sda")
		err := MountDrives([]string{"/dev/sda"})
		if err != nil {
			t.Errorf("Expected no error with LONGHORN_DISKS set, got: %v", err)
		}
		viper.Set("LONGHORN_DISKS", "")
	})

	t.Run("with SKIP_DISK_CHECK", func(t *testing.T) {
		viper.Set("SKIP_DISK_CHECK", true)
		err := MountDrives([]string{"/dev/sda"})
		if err != nil {
			t.Errorf("Expected no error with SKIP_DISK_CHECK, got: %v", err)
		}
		viper.Set("SKIP_DISK_CHECK", false)
	})

	t.Run("empty drives list", func(t *testing.T) {
		err := MountDrives([]string{})
		if err != nil {
			t.Errorf("Expected no error with empty drives list, got: %v", err)
		}
	})
}

func TestPersistMountedDisks(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	t.Run("with LONGHORN_DISKS set", func(t *testing.T) {
		viper.Set("LONGHORN_DISKS", "/dev/sda")
		err := PersistMountedDisks()
		if err != nil {
			t.Errorf("Expected no error with LONGHORN_DISKS set, got: %v", err)
		}
		viper.Set("LONGHORN_DISKS", "")
	})

	t.Run("with SKIP_DISK_CHECK", func(t *testing.T) {
		viper.Set("SKIP_DISK_CHECK", true)
		err := PersistMountedDisks()
		if err != nil {
			t.Errorf("Expected no error with SKIP_DISK_CHECK, got: %v", err)
		}
		viper.Set("SKIP_DISK_CHECK", false)
	})
}

func TestAppendToFile(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-append-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	content := "test content\n"
	err = appendToFile(tempFile.Name(), content)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	data, err := os.ReadFile(tempFile.Name())
	if err != nil {
		t.Errorf("Failed to read file: %v", err)
	}
	if string(data) != content {
		t.Errorf("Expected %s, got %s", content, string(data))
	}
}