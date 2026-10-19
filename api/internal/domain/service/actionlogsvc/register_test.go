package actionlogsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
)

// ADR 0018 step 2: an undo writes every snapshot back through the codec of
// its family. The composition root registers every service's module; a
// family or an entry kind two modules claim, or a codec that cannot restore,
// refuses the build instead of failing (or silently picking one) at the
// player's undo.

func noRestore(context.Context, int64, json.RawMessage) error { return nil }

func module(name string, families ...string) domain.Module {
	m := domain.Module{Name: name}
	for _, f := range families {
		m.Codecs = append(m.Codecs, codec{f, noRestore})
	}
	return m
}

func view(json.RawMessage) (json.RawMessage, error) { return json.RawMessage(`{}`), nil }

func TestRegister_RefusesWhatTwoModulesClaim(t *testing.T) {
	tests := []struct {
		name   string
		first  domain.Module
		second domain.Module
		names  []string
	}{
		{"a family", module("buffsvc", "buffs", "round"), module("castsvc", "round"), []string{`"round"`, "buffsvc", "castsvc"}},
		{"a family twice in one module", domain.Module{Name: "itemsvc"}, module("itemsvc", "items", "items"), []string{`"items"`, "itemsvc"}},
		{
			"an entry kind's payload view",
			domain.Module{Name: "combatsvc", Views: actionlog.PayloadViews{actionlog.KindSave: view}},
			domain.Module{Name: "turnsvc", Views: actionlog.PayloadViews{actionlog.KindSave: view}},
			[]string{string(actionlog.KindSave), "combatsvc", "turnsvc"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _, _ := newSvc(t)
			require.NoError(t, svc.Register(tc.first))
			err := svc.Register(tc.second)
			require.ErrorIs(t, err, actionlogsvc.ErrClaimed)
			for _, n := range tc.names {
				assert.Contains(t, err.Error(), n)
			}
		})
	}
}

func TestRegister_RefusesACodecThatCannotRestore(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	err := svc.Register(domain.Module{Name: "xpsvc", Codecs: []actionlog.Codec{
		actionlog.NewSnapshotter[int]("xp").MayBeAbsent().Restorer(func(context.Context, int64, string, int) error { return nil }, nil),
	}})
	require.ErrorIs(t, err, actionlog.ErrNoRestorer)
	assert.Contains(t, err.Error(), "xpsvc")
}

func TestRegister_RefusesAKeyThatIsNotAFamily(t *testing.T) {
	for _, family := range []string{"", "items:arrows"} {
		svc, _, _, _ := newSvc(t)
		assert.Error(t, svc.Register(module("itemsvc", family)), "family %q", family)
	}
}

// A refused module binds nothing: its other families stay free for the
// module that owns them.
func TestRegister_ARefusedModuleBindsNothing(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	require.NoError(t, svc.Register(module("buffsvc", "round")))
	require.ErrorIs(t, svc.Register(module("castsvc", "spell_slots", "round")), actionlogsvc.ErrClaimed)
	require.NoError(t, svc.Register(module("prepsvc", "spell_slots")), "spell_slots was not bound by the refused module")
}
