package router_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/dicesvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDice_RollWireShape(t *testing.T) {
	h, m := newRouter(t)
	m.dice.EXPECT().Roll(mock.Anything, int64(1), "2d6-d4+3").Return(&dicesvc.Result{
		Formula: "2d6-d4+3",
		Groups: []dicesvc.RolledGroup{
			{Sides: 6, Rolls: []int{2, 5}, Subtotal: 7},
			{Sides: 4, Rolls: []int{3}, Negative: true, Subtotal: -3},
		},
		Modifier: 3, Total: 7, LogEntryID: 12,
	}, nil)
	rec := do(h, "POST", "/api/roll/dice", `{"formula":"2d6-d4+3"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"formula":"2d6-d4+3","groups":[
		{"sides":6,"rolls":[2,5],"negative":false,"subtotal":7},
		{"sides":4,"rolls":[3],"negative":true,"subtotal":-3}],
		"modifier":3,"total":7,"logEntryId":12}`, rec.Body.String())
}

func TestDice_InvalidFormulaIs400WithTheField(t *testing.T) {
	h, m := newRouter(t)
	m.dice.EXPECT().Roll(mock.Anything, int64(1), "x").Return(nil, domain.FieldErrorf("formula", "bad"))
	rec := do(h, "POST", "/api/roll/dice", `{"formula":"x"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"bad","field":"formula"}`, rec.Body.String())
}

func TestDice_RequestParsing(t *testing.T) {
	for _, c := range []struct{ name, method, path, body, msg string }{
		{"roll bad json", "POST", "/api/roll/dice", "{", "invalid request body"},
		{"roll no formula", "POST", "/api/roll/dice", "{}", "formula is required"},
		{"roll unknown field", "POST", "/api/roll/dice", `{"formula":"d6","x":1}`, "invalid request body"},
		{"favourite no formula", "POST", "/api/dice/favourites", "{}", "formula is required"},
		{"delete bad id", "DELETE", "/api/dice/favourites/abc", "", "invalid favourite id"},
		{"delete zero id", "DELETE", "/api/dice/favourites/0", "", "invalid favourite id"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h, _ := newRouter(t)
			rec := do(h, c.method, c.path, c.body)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, fmt.Sprintf(`{"error":%q}`, c.msg), rec.Body.String())
		})
	}
}

func TestDice_FavouritesWireShape(t *testing.T) {
	h, m := newRouter(t)
	m.dice.EXPECT().Favourites(mock.Anything, int64(1)).Return([]dicesvc.Favourite{{ID: 1, Formula: "d20+5"}}, nil).Once()
	rec := do(h, "GET", "/api/dice/favourites", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"favourites":[{"id":1,"formula":"d20+5"}]}`, rec.Body.String())

	m.dice.EXPECT().Favourites(mock.Anything, int64(1)).Return(nil, nil).Once()
	rec = do(h, "GET", "/api/dice/favourites", "")
	assert.JSONEq(t, `{"favourites":[]}`, rec.Body.String())

	m.dice.EXPECT().AddFavourite(mock.Anything, int64(1), "1d20+5").Return(&dicesvc.Favourite{ID: 4, Formula: "d20+5"}, nil)
	rec = do(h, "POST", "/api/dice/favourites", `{"formula":"1d20+5"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"id":4,"formula":"d20+5"}`, rec.Body.String())

	m.dice.EXPECT().DeleteFavourite(mock.Anything, int64(1), int64(4)).Return(nil)
	m.dice.EXPECT().Favourites(mock.Anything, int64(1)).Return([]dicesvc.Favourite{{ID: 1, Formula: "d20+5"}}, nil).Once()
	rec = do(h, "DELETE", "/api/dice/favourites/4", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"favourites":[{"id":1,"formula":"d20+5"}]}`, rec.Body.String())

	m.dice.EXPECT().DeleteFavourite(mock.Anything, int64(1), int64(9)).Return(fmt.Errorf("%w: favourite 9", domain.ErrNotFound))
	rec = do(h, "DELETE", "/api/dice/favourites/9", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
