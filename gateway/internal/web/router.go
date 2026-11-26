package web

import (
	"embed"
	"net/http"
)

//go:embed assets
var assets embed.FS

// NewRouter обслуживает интерфейс и передаёт остальные запросы существующему API.
func NewRouter(api http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, assets, "assets/index.html")
	})
	for _, name := range []string{"app.css", "app.js"} {
		mux.HandleFunc("GET /assets/"+name, func(w http.ResponseWriter, r *http.Request) {
			http.ServeFileFS(w, r, assets, "assets/"+name)
		})
	}
	mux.Handle("/", api)
	return mux
}
