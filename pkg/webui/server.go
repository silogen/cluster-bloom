package webui

import (
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
)

// Server represents the web UI server
type Server struct {
	Port          int
	PortSpecified bool // true if user explicitly specified port via --port flag
}

// findAvailablePort finds an available port starting from startPort
func findAvailablePort(startPort int) int {
	for port := startPort; port < startPort+100; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			ln.Close()
			return port
		}
	}
	return startPort // fallback to original port if nothing available
}

// isPortAvailable checks if a specific port is available
func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// Start starts the web UI server
func (s *Server) Start() error {
	// If port was explicitly specified, fail if not available
	if s.PortSpecified {
		if !isPortAvailable(s.Port) {
			return fmt.Errorf("port %d is already in use", s.Port)
		}
	} else {
		// Auto-find available port starting from default
		availablePort := findAvailablePort(s.Port)
		if availablePort != s.Port {
			log.Printf("Port %d is in use, using port %d instead", s.Port, availablePort)
		}
		s.Port = availablePort
	}

	// Setup static file serving
	staticFS, err := fs.Sub(StaticFS, "web/static")
	if err != nil {
		return fmt.Errorf("failed to setup static filesystem: %w", err)
	}

	fileServer := http.FileServer(http.FS(staticFS))

	// Setup routes
	http.HandleFunc("/api/schema", handleSchema)
	http.HandleFunc("/api/generate", handleGenerate)
	http.HandleFunc("/api/save", handleSave)
	http.Handle("/", fileServer)

	addr := fmt.Sprintf(":%d", s.Port)
	fmt.Printf("🚀 Starting Cluster-Bloom Web Interface...\n")
	fmt.Printf("\n")
	fmt.Printf("🌐 Web interface starting on http://127.0.0.1:%d\n", s.Port)
	fmt.Printf("📊 Configuration interface accessible only from localhost\n")
	fmt.Printf("🔧 Configure your cluster at http://127.0.0.1:%d\n", s.Port)
	fmt.Printf("\n")
	fmt.Printf("🔗 For remote access, create an SSH tunnel:\n")
	fmt.Printf("   ssh -L %d:127.0.0.1:%d user@remote-server\n", s.Port, s.Port)
	fmt.Printf("   Then access: http://127.0.0.1:%d\n", s.Port)

	return http.ListenAndServe(addr, nil)
}
