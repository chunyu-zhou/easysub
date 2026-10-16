package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"easysub/internal/config"
	"easysub/internal/scheduler"
	"easysub/internal/server"
	"easysub/internal/store"
	"easysub/web"
)

const shutdownTimeout = 15 * time.Second

func main() {
	config.Load()

	webFS, err := fs.Sub(web.Files, "dist")
	if err != nil {
		log.Fatalf("[startup] unable to load embedded frontend: %v", err)
	}

	store.OnReady(scheduler.Start)
	app := server.New(context.Background(), webFS)
	httpServer := &http.Server{
		Addr:              config.C.Listen,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("[startup] EasySub %s listening on %s", server.Version, config.C.Listen)
		errCh <- httpServer.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-ctx.Done():
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[startup] HTTP server failed: %v", err)
		}
		return
	}

	log.Printf("[shutdown] stopping service")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[shutdown] graceful shutdown timed out: %v", err)
		if err := httpServer.Close(); err != nil {
			log.Printf("[shutdown] forced close failed: %v", err)
		}
	}
	scheduler.Stop()
	log.Printf("[shutdown] service stopped")
}
