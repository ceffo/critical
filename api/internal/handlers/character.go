package handlers

import (
	"database/sql"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain"
)

// CharacterHandler provides HTTP handlers for character-related operations.
type CharacterHandler struct {
	DB          *sql.DB
	CharacterID int64
}

// GetCharacter returns the core statistics for the character.
func (h *CharacterHandler) GetCharacter(w http.ResponseWriter, _ *http.Request) {
	actor, err := domain.LoadActor(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, actor.Stats)
}
