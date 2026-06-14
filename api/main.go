package handler

import "net/http"

// Main satisfies Vercel's need for an exported function in every file.
func Main(w http.ResponseWriter, r *http.Request) {
	Handler(w, r)
}
