// Package main is the entry point for the critical API.
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

	critdb "github.com/ceffo/critical/api/internal/db"
	"github.com/ceffo/critical/api/internal/handlers"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	db, err := critdb.Open("critical.db", "seeds")
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("error closing db: %v", err)
		}
	}()

	charID, err := getCharacterID(db)
	if err != nil {
		log.Fatalf("get character id: %v", err)
	}

	hp := &handlers.HPHandler{DB: db, CharacterID: charID}
	combat := &handlers.CombatHandler{DB: db, CharacterID: charID}
	buffs := &handlers.BuffsHandler{DB: db, CharacterID: charID}
	spells := &handlers.SpellsHandler{DB: db, CharacterID: charID}
	round := &handlers.RoundHandler{DB: db, CharacterID: charID}
	charH := &handlers.CharacterHandler{DB: db, CharacterID: charID}
	rolls := &handlers.RollsHandler{DB: db, CharacterID: charID}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			log.Printf("health check encode error: %v", err)
		}
	})

	r.Get("/api/character", charH.GetCharacter)

	r.Get("/api/hp", hp.GetHP)
	r.Post("/api/hp/event", hp.AppendEvent)
	r.Post("/api/hp/undo", hp.UndoLastEvent)

	r.Post("/api/roll/attack", combat.ResolveAttack)
	r.Post("/api/roll/save", rolls.RollSave)

	r.Get("/api/buffs", buffs.ListActive)
	r.Post("/api/buffs", buffs.Activate)
	r.Delete("/api/buffs/{id}", buffs.Dismiss)
	r.Post("/api/buffs/{id}/trigger", buffs.Trigger)

	r.Get("/api/spells/slots", spells.GetSlots)
	r.Get("/api/spells", spells.Search)
	r.Post("/api/spells/{id}/cast", spells.Cast)
	r.Post("/api/spells/reset", spells.Reset)

	r.Get("/api/round", round.GetRound)
	r.Post("/api/round/advance", round.Advance)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
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
