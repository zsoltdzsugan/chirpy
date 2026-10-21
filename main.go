package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func main() {
	const filepathRoot = "."
	const port = "8080"

	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
	}

	mux := http.NewServeMux()
	mux.Handle("GET /app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(filepathRoot)))))
	mux.Handle("app/assets/", http.FileServer(http.Dir("./assets")))

	// mux.HandleFunc("GET /api/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)

	mux.HandleFunc("GET /api/healthz", handlerReadiness)
	mux.HandleFunc("POST /api/validate_chirp", apiCfg.handlerValidateChirp)

	httpServer := &http.Server{
		Addr:           ":8080",
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Printf("Serving files from %s on port: %s\n", filepathRoot, port)
	httpServer.ListenAndServe()
}

func handlerReadiness(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Hits: %v\n", cfg.fileserverHits.Load())
	// hits := fmt.Sprintf("Hits: %v\n", cfg.fileserverHits.Load())
	// w.Write([]byte(hits))
	body := fmt.Sprintf(`
<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>
	`, cfg.fileserverHits.Load())
	w.Write([]byte(body))
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, req *http.Request) {
	cfg.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hits reset to 0"))
}

func (cfg *apiConfig) handlerValidateChirp(w http.ResponseWriter, req *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	type validResp struct {
		CleanedBody string `json:"cleaned_body"`
	}

	if len(params.Body) > 0 && len(params.Body) < 140 {
		cleanedBody := replaceWords(params.Body)
		respondWithJSON(w, 200, validResp{CleanedBody: cleanedBody})
	} else if len(params.Body) > 140 {
		respondWithError(w, 400, "Chirp is too long")
	}
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type invalidResp struct {
		Error string `json:"error"`
	}

	data := invalidResp{
		Error: msg,
	}

	dat, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshalling JSON %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)

}

func replaceWords(body string) string {
	replaceableWords := []string{"kerfuffle", "sharbert", "fornax"}

	for _, word := range replaceableWords {
		body = replaceWord(body, word, "****")
	}

	return body
}

func replaceWord(body string, word string, replacement string) string {
	parts := strings.Fields(body)

	for i, part := range parts {
		if strings.EqualFold(part, word) {
			parts[i] = replacement
		}
	}

	return strings.Join(parts, " ")
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, req)
	})
}
