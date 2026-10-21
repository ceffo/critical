package changelog_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/changelog"
)

func versions(n int) []changelog.Version {
	out := make([]changelog.Version, n)
	for i := range out {
		out[i] = changelog.Version{Version: fmt.Sprintf("1.%d.0", n-i), Features: []string{}, Fixes: []string{}}
	}
	return out
}

func names(p changelog.Page) []string {
	out := []string{}
	for _, v := range p.Versions {
		out = append(out, v.Version)
	}
	return out
}

// Step 6.2 and 4: pages follow the newest-first order, the cursor of one page starts the next, and the
// last page has no cursor.
func TestChangelog_Step4_PagesWalkTheVersionsNewestFirst(t *testing.T) {
	log, err := changelog.New(versions(5))
	require.NoError(t, err)

	p, err := log.Page("", 2)
	require.NoError(t, err)
	assert.Equal(t, []string{"1.5.0", "1.4.0"}, names(p))
	assert.Equal(t, "1.4.0", p.Next)

	p, err = log.Page(p.Next, 2)
	require.NoError(t, err)
	assert.Equal(t, []string{"1.3.0", "1.2.0"}, names(p))
	assert.Equal(t, "1.2.0", p.Next)

	p, err = log.Page(p.Next, 2)
	require.NoError(t, err)
	assert.Equal(t, []string{"1.1.0"}, names(p))
	assert.Empty(t, p.Next, "the last page has no cursor")

	p, err = log.Page("", 5)
	require.NoError(t, err)
	assert.Len(t, p.Versions, 5)
	assert.Empty(t, p.Next, "a page ending exactly at the end has no cursor")
}

// Step 4.2: a version with neither list is returned, with both empty.
func TestChangelog_Step4_AVersionWithoutChangesKeepsItsEmptyLists(t *testing.T) {
	log, err := changelog.New([]changelog.Version{{Version: "1.0.0", Features: []string{}, Fixes: []string{}}})
	require.NoError(t, err)
	p, err := log.Page("", 0)
	require.NoError(t, err)
	require.Len(t, p.Versions, 1)
	assert.Empty(t, p.Versions[0].Features)
	assert.Empty(t, p.Versions[0].Fixes)
}

// Step 6: the page size defaults and is capped; a dev build (nil or empty log) is an empty page, not an error.
func TestChangelog_Step6_LimitDefaultsIsCappedAndAnEmptyLogIsEmpty(t *testing.T) {
	log, err := changelog.New(versions(changelog.MaxLimit + 10))
	require.NoError(t, err)
	p, err := log.Page("", 0)
	require.NoError(t, err)
	assert.Len(t, p.Versions, changelog.DefaultLimit)
	p, err = log.Page("", 10_000)
	require.NoError(t, err)
	assert.Len(t, p.Versions, changelog.MaxLimit)
	assert.NotEmpty(t, p.Next)

	var none *changelog.Log
	p, err = none.Page("", 10)
	require.NoError(t, err)
	assert.Empty(t, p.Versions)
	assert.Empty(t, p.Next)
	empty, err := changelog.New(nil)
	require.NoError(t, err)
	p, err = empty.Page("", 10)
	require.NoError(t, err)
	assert.Empty(t, p.Versions)
}

// A cursor that names no version, a negative limit, a nameless or repeated version are refused as input errors.
func TestChangelog_Step6_RefusesBadInput(t *testing.T) {
	log, err := changelog.New(versions(2))
	require.NoError(t, err)
	_, err = log.Page("9.9.9", 1)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	_, err = log.Page("", -1)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	_, err = changelog.New([]changelog.Version{{Version: ""}})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	_, err = changelog.New([]changelog.Version{{Version: "1.0.0"}, {Version: "1.0.0"}})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}
