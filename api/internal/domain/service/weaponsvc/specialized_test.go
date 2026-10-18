package weaponsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc/mocks"
)

func character(specializes bool) *charactersvc.Character {
	return &charactersvc.Character{ID: 1, Derived: charactersvc.Derived{Specializes: specializes}}
}

// ADR 0030 step 12: a specialized weapon is a fighter's (Derived.Specializes);
// any other class is refused on field "specialized" with code not_allowed,
// before anything is written.
func TestCreate_Step12_SpecializedFightersOnly(t *testing.T) {
	in := weaponsvc.WeaponInput{Name: localized.Of("Long Sword"), DamageSM: "1d8", DamageL: "1d12", Kind: "melee", Specialized: true}

	t.Run("fighter", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		f := deps(t)
		svc, _ := weaponsvc.New(dal, f.Deps)
		f.chars.EXPECT().GetCharacter(mock.Anything, int64(1)).Return(character(true), nil)
		dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
		dal.EXPECT().Insert(mock.Anything, int64(1), mock.MatchedBy(func(w weaponsvc.Weapon) bool { return w.Specialized })).Return(nil)
		var payload []byte
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { payload = e.Payload }).Return(int64(3), nil)

		res, err := svc.Create(context.Background(), 1, in)
		require.NoError(t, err)
		assert.True(t, res.Weapon.Specialized)
		assert.Contains(t, string(payload), `"specialized":true`)
	})
	t.Run("bard", func(t *testing.T) {
		f := deps(t)
		svc, _ := weaponsvc.New(mocks.NewMockDAL(t), f.Deps) // nothing is written
		f.chars.EXPECT().GetCharacter(mock.Anything, int64(1)).Return(character(false), nil)

		_, err := svc.Create(context.Background(), 1, in)
		var fe *domain.FieldError
		require.True(t, errors.As(err, &fe), "got %v", err)
		assert.Equal(t, "specialized", fe.Field)
		assert.Equal(t, weaponsvc.CodeNotAllowed, fe.Message)
	})
	t.Run("not specialized reads no character", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		f := deps(t)
		svc, _ := weaponsvc.New(dal, f.Deps)
		dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
		dal.EXPECT().Insert(mock.Anything, int64(1), mock.Anything).Return(nil)
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(3), nil)
		plain := in
		plain.Specialized = false
		_, err := svc.Create(context.Background(), 1, plain)
		require.NoError(t, err)
	})
}

// ADR 0030 step 12: turning the flag on checks the class and logs the change;
// editing a weapon that is already specialized does not ask again.
func TestUpdate_Step12_SpecializedFlag(t *testing.T) {
	sword := weaponsvc.Weapon{ID: "sword", Name: localized.Of("Sword"), DamageSM: "1d8", DamageL: "1d12", Kind: "melee"}

	t.Run("refused for a bard", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		f := deps(t)
		svc, _ := weaponsvc.New(dal, f.Deps)
		dal.EXPECT().Get(mock.Anything, int64(1), "sword").Return(sword, nil)
		f.chars.EXPECT().GetCharacter(mock.Anything, int64(1)).Return(character(false), nil)
		_, err := svc.Update(context.Background(), 1, "sword", weaponsvc.WeaponPatch{Specialized: ptr(true)})
		var fe *domain.FieldError
		require.True(t, errors.As(err, &fe), "got %v", err)
		assert.Equal(t, "specialized", fe.Field)
	})
	t.Run("set for a fighter", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		f := deps(t)
		svc, _ := weaponsvc.New(dal, f.Deps)
		dal.EXPECT().Get(mock.Anything, int64(1), "sword").Return(sword, nil)
		f.chars.EXPECT().GetCharacter(mock.Anything, int64(1)).Return(character(true), nil)
		after := sword
		after.Specialized = true
		dal.EXPECT().Update(mock.Anything, int64(1), after).Return(nil)
		var got actionlog.NewEntry
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(4), nil)
		_, err := svc.Update(context.Background(), 1, "sword", weaponsvc.WeaponPatch{Specialized: ptr(true)})
		require.NoError(t, err)
		assert.Contains(t, string(got.Payload), `{"field":"specialized","before":"false","after":"true"}`)
		assert.Contains(t, string(got.Snapshots[0].After), `"specialized":true`)
	})
	t.Run("already specialized: the class is checked again", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		f := deps(t)
		svc, _ := weaponsvc.New(dal, f.Deps)
		f.chars.EXPECT().GetCharacter(mock.Anything, int64(1)).Return(character(true), nil)
		spec := sword
		spec.Specialized = true
		dal.EXPECT().Get(mock.Anything, int64(1), "sword").Return(spec, nil)
		dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Return(nil)
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(4), nil)
		_, err := svc.Update(context.Background(), 1, "sword", weaponsvc.WeaponPatch{MagicToHit: ptr(1)})
		require.NoError(t, err)
	})
}
