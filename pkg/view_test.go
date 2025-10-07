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
	"errors"
	"testing"
)

func TestStepResult(t *testing.T) {
	tests := []struct {
		name    string
		result  StepResult
		hasErr  bool
		message string
	}{
		{
			"success with message",
			StepResult{Error: nil, Message: "success"},
			false,
			"success",
		},
		{
			"success without message",
			StepResult{Error: nil, Message: ""},
			false,
			"",
		},
		{
			"error with message",
			StepResult{Error: errors.New("test error"), Message: "failed"},
			true,
			"failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.hasErr && tt.result.Error == nil {
				t.Errorf("Expected error but got none")
			} else if !tt.hasErr && tt.result.Error != nil {
				t.Errorf("Expected no error but got: %v", tt.result.Error)
			}
			if tt.result.Message != tt.message {
				t.Errorf("Expected message '%s', got '%s'", tt.message, tt.result.Message)
			}
		})
	}
}

func TestOptionResult(t *testing.T) {
	tests := []struct {
		name     string
		result   OptionResult
		canceled bool
		selected []string
		indexes  []int
	}{
		{
			"canceled result",
			OptionResult{Canceled: true, Selected: nil, Indexes: nil},
			true,
			nil,
			nil,
		},
		{
			"successful selection",
			OptionResult{
				Canceled: false,
				Selected: []string{"option1", "option2"},
				Indexes:  []int{0, 1},
			},
			false,
			[]string{"option1", "option2"},
			[]int{0, 1},
		},
		{
			"empty selection",
			OptionResult{Canceled: false, Selected: []string{}, Indexes: []int{}},
			false,
			[]string{},
			[]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.Canceled != tt.canceled {
				t.Errorf("Expected canceled=%v, got %v", tt.canceled, tt.result.Canceled)
			}
			if len(tt.result.Selected) != len(tt.selected) {
				t.Errorf("Expected %d selected items, got %d", len(tt.selected), len(tt.result.Selected))
			}
			if len(tt.result.Indexes) != len(tt.indexes) {
				t.Errorf("Expected %d indexes, got %d", len(tt.indexes), len(tt.result.Indexes))
			}
		})
	}
}

func TestStep(t *testing.T) {
	step := Step{
		Id:          "TestStep",
		Name:        "Test Step",
		Description: "A test step for unit testing",
		Action: func() StepResult {
			return StepResult{Error: nil, Message: "test completed"}
		},
	}

	if step.Id != "TestStep" {
		t.Errorf("Expected Id 'TestStep', got '%s'", step.Id)
	}
	if step.Name != "Test Step" {
		t.Errorf("Expected Name 'Test Step', got '%s'", step.Name)
	}
	if step.Description != "A test step for unit testing" {
		t.Errorf("Expected Description 'A test step for unit testing', got '%s'", step.Description)
	}
	if step.Action == nil {
		t.Errorf("Expected non-nil Action")
	}

	result := step.Action()
	if result.Error != nil {
		t.Errorf("Expected no error, got: %v", result.Error)
	}
	if result.Message != "test completed" {
		t.Errorf("Expected message 'test completed', got '%s'", result.Message)
	}
}

func TestLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		level LogLevel
		value int
	}{
		{"Debug level", Debug, 0},
		{"Info level", Info, 1},
		{"Warn level", Warn, 2},
		{"Error level", Error, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.level) != tt.value {
				t.Errorf("Expected value %d, got %d", tt.value, int(tt.level))
			}
		})
	}
}

func TestLogToUI(t *testing.T) {
	// Test when globalApp and globalLogView are nil
	LogToUI("test message")

	// Test when globalApp and globalLogView are set (would require UI setup)
	// This is mainly for coverage, actual UI testing would be more complex
}

func TestGlobalVariables(t *testing.T) {
	// Test initial state of global variables
	if globalApp != nil {
		t.Errorf("Expected globalApp to be nil initially")
	}
	if globalLogView != nil {
		t.Errorf("Expected globalLogView to be nil initially")
	}
	if ContinueOnFailure != false {
		t.Errorf("Expected ContinueOnFailure to be false initially")
	}
}

func TestStepStructure(t *testing.T) {
	// Test that Step struct has all required fields
	step := Step{}
	
	// Verify zero values
	if step.Id != "" {
		t.Errorf("Expected empty Id, got '%s'", step.Id)
	}
	if step.Name != "" {
		t.Errorf("Expected empty Name, got '%s'", step.Name)
	}
	if step.Description != "" {
		t.Errorf("Expected empty Description, got '%s'", step.Description)
	}
	if step.Action != nil {
		t.Errorf("Expected nil Action")
	}
}

func TestStepResultStructure(t *testing.T) {
	// Test StepResult struct
	result := StepResult{}
	
	// Verify zero values
	if result.Error != nil {
		t.Errorf("Expected nil Error, got %v", result.Error)
	}
	if result.Message != "" {
		t.Errorf("Expected empty Message, got '%s'", result.Message)
	}
}

func TestOptionResultStructure(t *testing.T) {
	// Test OptionResult struct
	result := OptionResult{}
	
	// Verify zero values
	if result.Selected != nil {
		t.Errorf("Expected nil Selected, got %v", result.Selected)
	}
	if result.Indexes != nil {
		t.Errorf("Expected nil Indexes, got %v", result.Indexes)
	}
	if result.Canceled != false {
		t.Errorf("Expected false Canceled, got %v", result.Canceled)
	}
}