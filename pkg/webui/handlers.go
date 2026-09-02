package webui

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/silogen/cluster-bloom/pkg/config"
)

func handleSchema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Load constraints from schema
	constraints, err := config.LoadConstraints()
	if err != nil {
		constraints = []config.ConstraintDef{} // Empty if loading fails
	}

	response := config.SchemaResponse{
		Arguments:   config.Schema(),
		Constraints: constraints,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req config.GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	prepared := config.PrepareGeneratedConfig(req.Config)

	// Validate before generating
	errors := config.Validate(prepared)
	if len(errors) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(config.ValidateResponse{
			Valid:  false,
			Errors: errors,
		})
		return
	}

	yaml := config.GenerateYAML(prepared)

	response := config.GenerateResponse{
		YAML: yaml,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req config.SaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate filename
	if req.Filename == "" {
		http.Error(w, "Filename is required", http.StatusBadRequest)
		return
	}

	// Validate before saving
	prepared := config.PrepareGeneratedConfig(req.Config)
	errors := config.Validate(prepared)
	if len(errors) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(config.ValidateResponse{
			Valid:  false,
			Errors: errors,
		})
		return
	}

	yaml := config.GenerateYAML(prepared)

	// Write to specified filename in current working directory
	if err := os.WriteFile(req.Filename, []byte(yaml), 0644); err != nil {
		http.Error(w, "Failed to write file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get absolute path for response
	cwd, _ := os.Getwd()

	response := map[string]interface{}{
		"success": true,
		"path":    cwd + "/" + req.Filename,
		"yaml":    yaml,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
