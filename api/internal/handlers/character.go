package handlers

import (
	"database/sql"
	"net/http"

	"github.com/moncef-naji/critical/api/internal/domain"
)

type CharacterHandler struct {
	DB          *sql.DB
	CharacterID int64
}

func (h *CharacterHandler) GetCharacter(w http.ResponseWriter, r *http.Request) {
	actor, err := domain.LoadActor(h.DB, h.CharacterID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, actor.Stats)
}
