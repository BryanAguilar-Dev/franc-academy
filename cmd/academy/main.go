// Comando academy: arranca el servidor de Franc Academy.
//
// Variables de entorno:
//
//	ADDR      dirección donde escuchar (por defecto :8081)
//	DATA_DIR  carpeta donde guardar progress.json (por defecto ./data)
//	TZ        zona horaria del reto diario (por defecto Europe/Madrid)
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	_ "time/tzdata" // incluye las zonas horarias: la imagen Docker no las trae

	academy "github.com/BryanAguilar-Dev/franc-academy"
	"github.com/BryanAguilar-Dev/franc-academy/internal/content"
	"github.com/BryanAguilar-Dev/franc-academy/internal/progress"
	"github.com/BryanAguilar-Dev/franc-academy/internal/server"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	addr := env("ADDR", ":8081")
	dataDir := env("DATA_DIR", "data")

	loc, err := time.LoadLocation(env("TZ", "Europe/Madrid"))
	if err != nil {
		log.Fatalf("zona horaria: %v", err)
	}
	cat, err := content.Load(academy.Content())
	if err != nil {
		log.Fatal(err)
	}
	store, err := progress.Open(filepath.Join(dataDir, "progress.json"))
	if err != nil {
		log.Fatal(err)
	}

	srv := &server.Server{
		Catalog: cat, Content: academy.Content(), Web: academy.Web(),
		Progress: store, Location: loc, Now: time.Now,
	}
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Apagado ordenado: al hacer "docker stop" terminamos las peticiones en curso.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpSrv.Shutdown(shutdown)
	}()

	log.Printf("Franc Academy: %d categorías, %d lecciones, escuchando en %s", len(cat.Categories), len(cat.Lessons), addr)
	if err := httpSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
