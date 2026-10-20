package ruletablesadpt

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// The language catalogue and the race_languages table (ADR 0021 step
// 14, ADR 0032 step 4): private JSON DTOs and the mapping; the entities and
// their validation live in ruletablesvc.

type languageDTO struct {
	Code string         `json:"code"`
	Name localized.Text `json:"name"`
}

type raceLanguageDTO struct {
	Race     string `json:"race"`
	Language string `json:"language"`
	Native   bool   `json:"native"`
}

// withLanguages adds the language catalogue (the rules are
// ruletablesvc.NewLanguages'); the list and the table are required.
func withLanguages(t *ruletablesvc.Tables, f fileDTO, raw []byte) (*ruletablesvc.Tables, error) {
	if len(raw) == 0 {
		return nil, errors.New("the artifact needs the languages list")
	}
	if len(f.RaceLanguages) == 0 {
		return nil, errors.New("the artifact needs the race_languages table")
	}
	var list []languageDTO
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&list); err != nil {
		return nil, fmt.Errorf("languages: parse: %w", err)
	}
	langs := make([]ruletablesvc.Language, len(list))
	for i, l := range list {
		langs[i] = ruletablesvc.Language{Code: l.Code, Name: l.Name.Clean()}
	}
	rows := make([]ruletablesvc.RaceLanguageRow, len(f.RaceLanguages))
	for i, r := range f.RaceLanguages {
		rows[i] = ruletablesvc.RaceLanguageRow(r)
	}
	l, err := ruletablesvc.NewLanguages(langs, rows)
	if err != nil {
		return nil, err
	}
	return t.WithLanguages(l), nil
}
