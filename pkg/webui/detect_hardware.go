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
