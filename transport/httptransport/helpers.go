package httptransport

import (
	"compress/gzip"
	"encoding/json"
	"net/http"
	"strings"
)

// respondWithJSON responds to an HTTP request with a JSON payload
func respondWithJSON(w http.ResponseWriter, r *http.Request, code int, payload interface{}, options *ServerOptions) {
	response, err := json.Marshal(payload)
	if err != nil {
		respondWithError(w, r, http.StatusInternalServerError, "failed to marshal response", options)
		return
	}

	// Check if compression should be used
	useCompression := false
	if options != nil && options.CompressionEnabled && 
	   len(response) >= int(options.CompressionThreshold) {
		// Check if client accepts gzip
		acceptEncoding := r.Header.Get("Accept-Encoding")
		if strings.Contains(acceptEncoding, "gzip") {
			useCompression = true
		}
	}

	w.Header().Set("Content-Type", "application/json")
	
	if useCompression {
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(code)
		
		gz := gzip.NewWriter(w)
		defer gz.Close()
		gz.Write(response)
	} else {
		w.WriteHeader(code)
		w.Write(response)
	}
}

// respondWithError responds to an HTTP request with an error message
func respondWithError(w http.ResponseWriter, r *http.Request, code int, message string, options *ServerOptions) {
	respondWithJSON(w, r, code, map[string]string{"error": message}, options)
}
