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
	"sync"
	"time"
)

type WebLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Step      string    `json:"step"`
}

type WebVariable struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
	Type  string      `json:"type"`
}

type WebStepStatus struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Duration    string    `json:"duration"`
	Error       string    `json:"error,omitempty"`
	Progress    int       `json:"progress"`
}

type WebMonitor struct {
	logs      []WebLogEntry
	variables map[string]WebVariable
	steps     map[string]*WebStepStatus
	mutex     sync.RWMutex
}

func NewWebMonitor() *WebMonitor {
	return &WebMonitor{
		logs:      make([]WebLogEntry, 0),
		variables: make(map[string]WebVariable),
		steps:     make(map[string]*WebStepStatus),
	}
}

func (m *WebMonitor) AddLog(level, message, step string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	entry := WebLogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Step:      step,
	}

	m.logs = append(m.logs, entry)

	if len(m.logs) > 200 {
		m.logs = m.logs[1:]
	}
}

func (m *WebMonitor) SetVariable(name string, value interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var valueType string
	switch value.(type) {
	case int, int64, int32:
		valueType = "integer"
	case float32, float64:
		valueType = "float"
	case string:
		valueType = "string"
	case bool:
		valueType = "boolean"
	default:
		valueType = "unknown"
	}

	m.variables[name] = WebVariable{
		Name:  name,
		Value: value,
		Type:  valueType,
	}
}

func (m *WebMonitor) InitializeStep(step Step, progress int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.steps[step.Id] = &WebStepStatus{
		ID:          step.Id,
		Name:        step.Name,
		Description: step.Description,
		Status:      "pending",
		Progress:    progress,
	}
}

func (m *WebMonitor) StartStep(stepId string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if stepStatus, exists := m.steps[stepId]; exists {
		stepStatus.Status = "running"
		stepStatus.StartTime = time.Now()
	}
}

func (m *WebMonitor) CompleteStep(stepId string, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if stepStatus, exists := m.steps[stepId]; exists {
		stepStatus.EndTime = time.Now()
		if err != nil {
			stepStatus.Status = "failed"
			stepStatus.Error = err.Error()
		} else {
			stepStatus.Status = "completed"
			stepStatus.Error = ""
		}
		stepStatus.Duration = stepStatus.EndTime.Sub(stepStatus.StartTime).Round(time.Millisecond).String()
	}
}

func (m *WebMonitor) SkipStep(stepId string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if stepStatus, exists := m.steps[stepId]; exists {
		stepStatus.Status = "skipped"
		stepStatus.EndTime = time.Now()
		stepStatus.Duration = "0s"
	}
}

func (m *WebMonitor) GetLogs() []WebLogEntry {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	logs := make([]WebLogEntry, len(m.logs))
	for i, j := 0, len(m.logs)-1; j >= 0; i, j = i+1, j-1 {
		logs[i] = m.logs[j]
	}
	return logs
}

func (m *WebMonitor) GetVariables() map[string]WebVariable {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]WebVariable)
	for k, v := range m.variables {
		result[k] = v
	}
	return result
}

func (m *WebMonitor) GetSteps() map[string]*WebStepStatus {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]*WebStepStatus)
	for k, v := range m.steps {
		result[k] = v
	}
	return result
}

func (m *WebMonitor) IsCompleted() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, step := range m.steps {
		if step.Status == "pending" || step.Status == "running" {
			return false
		}
	}
	return true
}

func (m *WebMonitor) HasErrors() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, step := range m.steps {
		if step.Status == "failed" {
			return true
		}
	}
	return false
}