package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseHeaderLinks(t *testing.T) {
	for _, raw := range []string{"", " ", "[]", "null", "invalid", "{}", `[1]`} {
		t.Run(raw, func(t *testing.T) {
			require.Equal(t, []HeaderLink{}, ParseHeaderLinks(raw))
			require.Equal(t, []HeaderLink{}, ParseUserVisibleHeaderLinks(raw))
		})
	}
	raw := `[{"id":"help","label":"Help","url":"/help","icon_svg":"<svg/>","visibility":"user","open_in_new_tab":true,"sort_order":2},{"id":"admin","label":"Admin","url":"/admin","visibility":"admin","open_in_new_tab":false,"sort_order":1},{"id":"legacy","label":"Legacy","url":"/"}]`
	all := ParseHeaderLinks(raw)
	require.Len(t, all, 3)
	require.True(t, all[0].OpenInNewTab)
	require.False(t, all[1].OpenInNewTab)
	require.False(t, all[2].OpenInNewTab)
	require.Equal(t, []HeaderLink{all[0], all[2]}, ParseUserVisibleHeaderLinks(raw))
	require.Empty(t, ParseUserVisibleHeaderLinks(`[{"visibility":"admin"}]`))
}

func TestCustomMenuOpenInNewTabJSON(t *testing.T) {
	items := ParseCustomMenuItems(`[{"id":"legacy"},{"id":"enabled","open_in_new_tab":true},{"id":"disabled","open_in_new_tab":false}]`)
	require.Len(t, items, 3)
	require.False(t, items[0].OpenInNewTab)
	require.True(t, items[1].OpenInNewTab)
	require.False(t, items[2].OpenInNewTab)
	encoded, err := json.Marshal(items)
	require.NoError(t, err)
	var fields []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &fields))
	require.NotContains(t, fields[0], "open_in_new_tab")
	require.Equal(t, "true", string(fields[1]["open_in_new_tab"]))
	require.NotContains(t, fields[2], "open_in_new_tab")
	require.Equal(t, items, ParseCustomMenuItems(string(encoded)))
}
