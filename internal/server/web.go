package server

import "net/http"

func registerWebRoutes(mux *http.ServeMux) {
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("web/assets"))))
	for route, page := range map[string]string{
		"/devices": "devices", "/history": "history", "/connection": "connection",
		"/sites": "sites", "/settings/wifi": "wifi",
	} {
		mux.HandleFunc("GET "+route, func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "web/pages/"+page+".html")
		})
	}
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/devices", http.StatusFound)
	})
}
