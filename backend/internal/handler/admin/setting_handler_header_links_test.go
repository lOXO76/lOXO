package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettingHandlerHeaderLinksValidation(t *testing.T) {
	valid := dto.HeaderLink{ID: "help_1", Label: "帮助中心", URL: "/help", Visibility: "user", OpenInNewTab: true}
	for _, test := range []struct {
		name    string
		change  func(*dto.HeaderLink)
		message string
	}{
		{name: "missing label", change: func(link *dto.HeaderLink) { link.Label = " \t\n" }, message: "Header link label is required"},
		{name: "long label", change: func(link *dto.HeaderLink) { link.Label = strings.Repeat("字", 31) }, message: "Header link label is too long"},
		{name: "missing URL", change: func(link *dto.HeaderLink) { link.URL = "" }, message: "Header link URL is required"},
		{name: "javascript URL", change: func(link *dto.HeaderLink) { link.URL = "javascript:alert(1)" }, message: "Header link URL must be"},
		{name: "protocol relative URL", change: func(link *dto.HeaderLink) { link.URL = "//evil.com" }, message: "Header link URL must be"},
		{name: "relative URL", change: func(link *dto.HeaderLink) { link.URL = "help/start" }, message: "Header link URL must be"},
		{name: "markdown URL", change: func(link *dto.HeaderLink) { link.URL = "md:help" }, message: "Header link URL must be"},
		{name: "ftp URL", change: func(link *dto.HeaderLink) { link.URL = "ftp://example.com" }, message: "Header link URL must be"},
		{name: "missing host", change: func(link *dto.HeaderLink) { link.URL = "https://" }, message: "Header link URL must be"},
		{name: "external fragment", change: func(link *dto.HeaderLink) { link.URL = "https://example.com/#help" }, message: "Header link URL must be"},
		{name: "long URL", change: func(link *dto.HeaderLink) { link.URL = "/" + strings.Repeat("a", 2048) }, message: "Header link URL is too long"},
		{name: "path whitespace", change: func(link *dto.HeaderLink) { link.URL = "/help center" }, message: "Header link URL must be"},
		{name: "path tab", change: func(link *dto.HeaderLink) { link.URL = "/help\tcenter" }, message: "Header link URL must be"},
		{name: "path unicode whitespace", change: func(link *dto.HeaderLink) { link.URL = "/help\u3000center" }, message: "Header link URL must be"},
		{name: "path control", change: func(link *dto.HeaderLink) { link.URL = "/help\x00center" }, message: "Header link URL must be"},
		{name: "path backslash origin", change: func(link *dto.HeaderLink) { link.URL = "/\\evil.com" }, message: "Header link URL must be"},
		{name: "external whitespace", change: func(link *dto.HeaderLink) { link.URL = "https://example.com/help center" }, message: "Header link URL must be"},
		{name: "leading whitespace", change: func(link *dto.HeaderLink) { link.URL = " /help" }, message: "Header link URL must be"},
		{name: "invalid visibility", change: func(link *dto.HeaderLink) { link.Visibility = "everyone" }, message: "Header link visibility must be 'user' or 'admin'"},
		{name: "missing visibility", change: func(link *dto.HeaderLink) { link.Visibility = "" }, message: "Header link visibility must be"},
		{name: "long icon", change: func(link *dto.HeaderLink) { link.IconSVG = strings.Repeat("a", 10*1024+1) }, message: "Header link icon SVG is too large"},
		{name: "long ID", change: func(link *dto.HeaderLink) { link.ID = strings.Repeat("a", 33) }, message: "Header link ID is too long"},
		{name: "invalid ID", change: func(link *dto.HeaderLink) { link.ID = "help/1" }, message: "Header link ID contains invalid characters"},
		{name: "unicode ID", change: func(link *dto.HeaderLink) { link.ID = "帮助" }, message: "Header link ID contains invalid characters"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, repo := newStepUpSwitchTestHandler(t, map[string]string{service.SettingKeyHeaderLinks: "[]"})
			link := valid
			test.change(&link)
			recorder := doUpdateSettings(t, handler, map[string]any{"header_links": []dto.HeaderLink{link}}, nil)
			require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			require.Contains(t, recorder.Body.String(), test.message)
			require.Equal(t, "[]", repo.values[service.SettingKeyHeaderLinks])
		})
	}
	for _, test := range []struct {
		name    string
		links   []dto.HeaderLink
		message string
	}{
		{name: "too many links", links: make([]dto.HeaderLink, 11), message: "Too many header links (max 10)"},
		{name: "duplicate IDs", links: []dto.HeaderLink{valid, valid}, message: "Duplicate header link ID: help_1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, _ := newStepUpSwitchTestHandler(t, map[string]string{})
			recorder := doUpdateSettings(t, handler, map[string]any{"header_links": test.links}, nil)
			require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			require.Contains(t, recorder.Body.String(), test.message)
		})
	}
}

func TestSettingHandlerHeaderLinksRoundTrip(t *testing.T) {
	handler, repo := newStepUpSwitchTestHandler(t, map[string]string{})
	links := []dto.HeaderLink{
		{ID: "Help-1", Label: "  帮助中心  ", URL: "https://example.com/help", IconSVG: "<svg></svg>", Visibility: "user", OpenInNewTab: true, SortOrder: 3},
		{Label: strings.Repeat("字", 30), URL: "/admin/settings?tab=general#links", Visibility: "admin", SortOrder: -1},
		{ID: " ", Label: "首页", URL: "/", Visibility: "user"},
	}
	for len(links) < 10 {
		links = append(links, dto.HeaderLink{Label: "站内链接", URL: "/help?query=%20", Visibility: "user"})
	}
	links[3].ID = strings.Repeat("a", 32)
	links[3].IconSVG = strings.Repeat("a", 10*1024)
	links[3].URL = "/" + strings.Repeat("a", 2047)
	recorder := doUpdateSettings(t, handler, map[string]any{"header_links": links}, nil)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var updated struct {
		Data dto.SystemSettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &updated))
	require.Len(t, updated.Data.HeaderLinks, 10)
	require.Equal(t, "帮助中心", updated.Data.HeaderLinks[0].Label)
	require.True(t, updated.Data.HeaderLinks[0].OpenInNewTab)
	require.False(t, updated.Data.HeaderLinks[1].OpenInNewTab)
	seen := make(map[string]bool)
	for _, link := range updated.Data.HeaderLinks {
		require.NotEmpty(t, link.ID)
		require.LessOrEqual(t, len(link.ID), 32)
		require.Regexp(t, menuItemIDPattern, link.ID)
		require.False(t, seen[link.ID])
		seen[link.ID] = true
	}
	require.Equal(t, updated.Data.HeaderLinks, dto.ParseHeaderLinks(repo.values[service.SettingKeyHeaderLinks]))
	getRecorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(getRecorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	handler.GetSettings(ctx)
	require.Equal(t, http.StatusOK, getRecorder.Code)
	var fetched struct {
		Data dto.SystemSettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(getRecorder.Body.Bytes(), &fetched))
	require.Equal(t, updated.Data.HeaderLinks, fetched.Data.HeaderLinks)
	settings, err := handler.settingService.GetAllSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, repo.values[service.SettingKeyHeaderLinks], settings.HeaderLinks)
}

func TestSettingHandlerHeaderLinksPartialUpdates(t *testing.T) {
	stored := `[{"id":"help","label":"Help","url":"/help","visibility":"admin","open_in_new_tab":true,"sort_order":3}]`
	for _, test := range []struct {
		name string
		body map[string]any
		want string
	}{
		{name: "omitted", body: map[string]any{"site_name": "Updated"}, want: stored},
		{name: "null", body: map[string]any{"header_links": nil}, want: stored},
		{name: "clear", body: map[string]any{"header_links": []dto.HeaderLink{}}, want: "[]"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, repo := newStepUpSwitchTestHandler(t, map[string]string{service.SettingKeyHeaderLinks: stored})
			recorder := doUpdateSettings(t, handler, test.body, nil)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, test.want, repo.values[service.SettingKeyHeaderLinks])
		})
	}
}

func TestSettingHandlerCustomMenuOpenInNewTabRoundTrip(t *testing.T) {
	handler, repo := newStepUpSwitchTestHandler(t, map[string]string{})
	items := []dto.CustomMenuItem{
		{ID: "external", Label: "Help", URL: "https://example.com", Visibility: "user", OpenInNewTab: true},
		{ID: "markdown", Label: "Guide", URL: "md:guide", Visibility: "admin", OpenInNewTab: true},
		{ID: "embedded", Label: "Embedded", URL: "https://example.org", Visibility: "user"},
	}
	recorder := doUpdateSettings(t, handler, map[string]any{"custom_menu_items": items}, nil)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, items, dto.ParseCustomMenuItems(repo.values[service.SettingKeyCustomMenuItems]))
	var response struct {
		Data dto.SystemSettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, items, response.Data.CustomMenuItems)
	getRecorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(getRecorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	handler.GetSettings(ctx)
	require.NoError(t, json.Unmarshal(getRecorder.Body.Bytes(), &response))
	require.Equal(t, items, response.Data.CustomMenuItems)
	stored := repo.values[service.SettingKeyCustomMenuItems]
	recorder = doUpdateSettings(t, handler, map[string]any{"header_links": []dto.HeaderLink{}}, nil)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, stored, repo.values[service.SettingKeyCustomMenuItems])
	legacy := dto.ParseCustomMenuItems(`[{"id":"legacy","label":"Legacy","url":"https://example.com","visibility":"user"}]`)
	require.False(t, legacy[0].OpenInNewTab)
}

func TestDiffSettingsHeaderLinks(t *testing.T) {
	before := &service.SystemSettings{HeaderLinks: "[]"}
	after := *before
	require.NotContains(t, diffSettings(before, &after, nil, nil, UpdateSettingsRequest{}), service.SettingKeyHeaderLinks)
	after.HeaderLinks = `[{"id":"help"}]`
	require.Equal(t, []string{service.SettingKeyHeaderLinks}, diffSettings(before, &after, nil, nil, UpdateSettingsRequest{}))
}
