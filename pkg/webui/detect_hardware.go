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

package webui

import (
	"encoding/json"
	"net/http"

	"github.com/silogen/cluster-bloom/pkg/config"
)

func handleDetectHardware(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	scan := config.ScanHostHardware()
	response := config.DetectHardwareResponse{
		AIMHardwareFamily: config.FormatAIMHardwareFamily(scan.Detected),
		GPUFamilies:       scan.Detected.GPU.Families,
		EPYCModel:         scan.Detected.EPYCModel,
		Warnings:          scan.Warnings(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
