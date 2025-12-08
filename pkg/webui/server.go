package webui

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
)

// Server represents the web UI server
type Server struct {
	Port int
}

// Start starts the web UI server
func (s *Server) Start() error {
	// Setup static file serving
	staticFS, err := fs.Sub(StaticFS, "web/static")
	if err != nil {
		return fmt.Errorf("failed to setup static filesystem: %w", err)
	}

	fileServer := http.FileServer(http.FS(staticFS))

	// Setup routes
	http.HandleFunc("/api/schema", handleSchema)
	http.HandleFunc("/api/validate", handleValidate)
	http.HandleFunc("/api/generate", handleGenerate)
	http.Handle("/", fileServer)

	addr := fmt.Sprintf(":%d", s.Port)
	log.Printf("Starting Bloom Web UI at http://localhost%s", addr)
	log.Printf("Press Ctrl+C to stop")

	return http.ListenAndServe(addr, nil)
}
