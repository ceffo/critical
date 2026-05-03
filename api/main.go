package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	critdb "github.com/moncef-naji/critical/api/internal/db"
	"github.com/moncef-naji/critical/api/internal/handlers"
)

func main() {
	db, err := critdb.Open("critical.db", "seeds")
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	charID, err := getCharacterID(db)
	if err != nil {
		log.Fatalf("get character id: %v", err)
	}

	hp := &handlers.HPHandler{DB: db, CharacterID: charID}
	combat := &handlers.CombatHandler{DB: db, CharacterID: charID}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	r.Get("/api/hp", hp.GetHP)
	r.Post("/api/hp/event", hp.AppendEvent)
	r.Post("/api/hp/undo", hp.UndoLastEvent)

	r.Post("/api/roll/attack", combat.ResolveAttack)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go func() {
		log.Println("api listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
	log.Println("server stopped")
}

func getCharacterID(db *sql.DB) (int64, error) {
	var id int64
	if err := db.QueryRow("SELECT id FROM characters LIMIT 1").Scan(&id); err != nil {
		return 0, fmt.Errorf("no character seeded: %w", err)
	}
	return id, nil
}
