package server

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"
)

//go:embed templates/stats.html
var templateFS embed.FS

// funcMap defines template helper functions
var funcMap = template.FuncMap{
	"ge": func(a, b float64) bool {
		return a >= b
	},
}

// renderStatsHTML renders the stats page using the HTML template
func renderStatsHTML(w http.ResponseWriter, stats map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html")

	htmlBytes, err := templateFS.ReadFile("templates/stats.html")
	if err != nil {
		slog.Error("Error reading embedded template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.New("stats").Funcs(funcMap).Parse(string(htmlBytes))
	if err != nil {
		slog.Error("Error parsing HTML template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, stats); err != nil {
		slog.Error("Error executing HTML template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

