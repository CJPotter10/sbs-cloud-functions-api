package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	cloudfunctions "github.com/CJPotter10/sbs-cloud-functions-api/cloud-functions"
	"github.com/CJPotter10/sbs-cloud-functions-api/utils"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

func main() {

	port := "8080"

	if fromEnv := os.Getenv("PORT"); fromEnv != "" {
		port = fromEnv
	}

	utils.NewDatabaseClient()

	fmt.Printf("Starting up on http://localhost:%s\n", port)

	r := chi.NewRouter()

	r.Use(middleware.Logger)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello World"))
	})

	r.Post("/calculateADP", cloudfunctions.CalculateADP)
	r.Post("/scoreDraftTokens", cloudfunctions.ScoreDraftTokensEndPoint)

	log.Fatal(http.ListenAndServe(":"+port, r))
}
