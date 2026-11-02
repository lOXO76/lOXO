//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type creationCatalogGroupStub struct {
	creationGroupSource
	groups  []Group
	err     error
	calls   int
	ctx     context.Context
	userID  int64
	rates   map[int64]float64
	rateErr error
}

func (stub *creationCatalogGroupStub) GetUserGroupRates(context.Context, int64) (map[int64]float64, error) {
	return stub.rates, stub.rateErr
}

func (stub *creationCatalogGroupStub) GetAvailableGroups(ctx context.Context, userID int64) ([]Group, error) {
	stub.calls++
	stub.ctx, stub.userID = ctx, userID
	return stub.groups, stub.err
}

type creationCatalogAccountStub struct {
	AccountRepository
	accounts  []Account
	err       error
	calls     int
	ctx       context.Context
	platforms []string
}

func (stub *creationCatalogAccountStub) ListSchedulableByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	stub.calls++
	stub.ctx = ctx
	stub.platforms = append([]string(nil), platforms...)
	return stub.accounts, stub.err
}

type creationCatalogTaskStore struct {
	ImageTaskStore
}

func creationCatalogTestGroup(platform string) *Group {
	return &Group{
		ID: 7, Name: "创作分组", Description: "图片和视频", Platform: platform,
		Status: StatusActive, SubscriptionType: "standard", RateMultiplier: 1.25,
		AllowImageGeneration: true,
	}
}

func creationCatalogTestAccount(platform string, mapping map[string]any) *Account {
	return &Account{
		ID: 91, Name: "private-account", Platform: platform, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{7},
		Credentials: map[string]any{
			"api_key": "private-credential", "base_url": "https://private-upstream.example/v1",
			"model_mapping": mapping,
		},
	}
}

func creationCatalogTestVideoAccount(protocol string, mapping map[string]any) *Account {
	account := creationCatalogTestAccount(PlatformVideo, mapping)
	account.Credentials["video_protocol"] = protocol
	return account
}

func creationCatalogTestSeedanceAccount(mapping map[string]any) *Account {
	account := creationCatalogTestAccount(PlatformOpenAI, mapping)
	account.Credentials["openai_capabilities"] = []string{"seedance"}
	return account
}

func TestCreationCatalogBuildGroupCapabilities(t *testing.T) {
	for _, test := range []struct {
		name           string
		platform       string
		imageDisabled  bool
		asyncEnabled   bool
		accounts       []*Account
		imageProtocol  string
		videoProtocols []string
		asyncSupported bool
	}{
		{name: "image only openai", platform: PlatformOpenAI, imageProtocol: "openai"},
		{name: "openai async", platform: PlatformOpenAI, asyncEnabled: true, imageProtocol: "openai", asyncSupported: true},
		{name: "gemini async gated", platform: PlatformGemini, asyncEnabled: true, imageProtocol: "gemini"},
		{name: "gemini image", platform: PlatformGemini, imageProtocol: "gemini"},
		{name: "grok image and video", platform: PlatformGrok, asyncEnabled: true,
			accounts: []*Account{creationCatalogTestAccount(PlatformGrok, nil)}, imageProtocol: "openai",
			videoProtocols: []string{"grok"}, asyncSupported: true},
		{name: "grok async disabled", platform: PlatformGrok, imageProtocol: "openai"},
		{name: "grok image disabled also gates video", platform: PlatformGrok, imageDisabled: true,
			accounts: []*Account{creationCatalogTestAccount(PlatformGrok, nil)}},
		{name: "openai image and seedance", platform: PlatformOpenAI, imageProtocol: "openai",
			accounts: []*Account{creationCatalogTestSeedanceAccount(nil)}, videoProtocols: []string{"seedance"}},
		{name: "image disabled keeps video", platform: PlatformOpenAI, imageDisabled: true,
			accounts: []*Account{creationCatalogTestSeedanceAccount(nil)}, videoProtocols: []string{"seedance"}},
		{name: "image disabled without video", platform: PlatformOpenAI, imageDisabled: true},
		{name: "video only", platform: PlatformVideo, asyncEnabled: true,
			accounts: []*Account{creationCatalogTestVideoAccount(VideoProtocolSeedance, nil)}, videoProtocols: []string{"seedance"}},
		{name: "video without accounts", platform: PlatformVideo},
		{name: "composite unsupported", platform: PlatformComposite,
			accounts: []*Account{creationCatalogTestSeedanceAccount(nil)}},
		{name: "anthropic unsupported", platform: PlatformAnthropic},
		{name: "antigravity unsupported", platform: PlatformAntigravity},
		{name: "unknown video protocol", platform: PlatformVideo,
			accounts: []*Account{creationCatalogTestVideoAccount("unknown", nil)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			group := creationCatalogTestGroup(test.platform)
			group.AllowImageGeneration = !test.imageDisabled
			entry := buildCreationGroup(group, test.accounts, nil, test.asyncEnabled)
			require.Equal(t, group.ID, entry.ID)
			require.Equal(t, group.Name, entry.Name)
			require.Equal(t, group.Description, entry.Description)
			require.Equal(t, group.Platform, entry.Platform)
			require.Equal(t, group.SubscriptionType, entry.SubscriptionType)
			require.Equal(t, group.RateMultiplier, entry.RateMultiplier)
			if test.imageProtocol == "" {
				require.Nil(t, entry.Image)
			} else {
				require.NotNil(t, entry.Image)
				require.Equal(t, []string{test.imageProtocol}, entry.Image.Protocols)
				require.Equal(t, test.asyncSupported, entry.Image.AsyncSupported)
				require.True(t, entry.Image.EditSupported)
				require.NotNil(t, entry.Image.Models)
			}
			if len(test.videoProtocols) == 0 {
				require.Nil(t, entry.Video)
			} else {
				require.NotNil(t, entry.Video)
				require.Equal(t, test.videoProtocols, entry.Video.Protocols)
				require.NotNil(t, entry.Video.Models)
			}
		})
	}
}

func TestCreationCatalogImageModelSources(t *testing.T) {
	group := creationCatalogTestGroup(PlatformOpenAI)
	group.ModelPricing = []ChannelModelPricing{
		{Platform: PlatformOpenAI, BillingMode: BillingModeImage, Models: []string{"group-image", "duplicate"}},
		{Platform: PlatformOpenAI, BillingMode: BillingModeToken, Models: []string{"token-model"}},
		{Platform: PlatformOpenAI, BillingMode: BillingModePerRequest, Models: []string{"request-model"}},
		{Platform: PlatformGemini, BillingMode: BillingModeImage, Models: []string{"foreign-group-image"}},
	}
	channel := &Channel{
		Status: StatusActive,
		ModelPricing: []ChannelModelPricing{
			{Platform: PlatformOpenAI, BillingMode: BillingModeImage, Models: []string{"channel-image", "duplicate", " ", "gpt-image-*", "image?", "image[12]"}},
			{Platform: PlatformGemini, BillingMode: BillingModeImage, Models: []string{"foreign-channel-image"}},
		},
		ModelMapping: map[string]map[string]string{
			PlatformOpenAI: {"channel-alias": "channel-image", "channel-native": "gpt-image-2", "wildcard-*": "gpt-image-2"},
			PlatformGemini: {"foreign-alias": "gpt-image-2"},
		},
	}
	account := creationCatalogTestAccount(PlatformOpenAI, map[string]any{
		"account-image": "gpt-image-2", "duplicate": "gpt-image-2", "plain-chat": "gpt-4.1",
		"account-*": "gpt-image-2", "": "gpt-image-2",
	})
	entry := buildCreationGroup(group, []*Account{account}, channel, false)
	require.NotNil(t, entry.Image)
	require.Equal(t, []CreationModel{
		{ID: "account-image", Protocols: []string{"openai"}},
		{ID: "channel-alias", Protocols: []string{"openai"}},
		{ID: "channel-image", Protocols: []string{"openai"}},
		{ID: "channel-native", Protocols: []string{"openai"}},
		{ID: "duplicate", Protocols: []string{"openai"}},
		{ID: "group-image", Protocols: []string{"openai"}},
	}, entry.Image.Models)
	require.Nil(t, entry.Video)
}

func TestCreationCatalogVideoModelSources(t *testing.T) {
	group := creationCatalogTestGroup(PlatformVideo)
	group.ModelPricing = []ChannelModelPricing{
		{Platform: PlatformVideo, BillingMode: BillingModeVideo, Models: []string{"seedance-group", "seedance-duplicate"}},
		{Platform: PlatformOpenAI, BillingMode: BillingModeVideo, Models: []string{"foreign-group-video"}},
	}
	group.VideoModelPrices = map[string]map[string]float64{
		"seedance-per-model": {"720p": 0.01}, "seedance-duplicate": {"720p": 0.02}, "seedance-*": {"720p": 0.03},
	}
	channel := &Channel{
		Status: StatusActive,
		ModelPricing: []ChannelModelPricing{
			{Platform: PlatformVideo, BillingMode: BillingModeVideo, Models: []string{"seedance-channel", "seedance-duplicate"}},
			{Platform: PlatformGemini, BillingMode: BillingModeVideo, Models: []string{"foreign-video"}},
		},
		ModelMapping: map[string]map[string]string{
			PlatformVideo:  {"channel-alias": "seedance-channel"},
			PlatformOpenAI: {"foreign-alias": "seedance-channel"},
		},
	}
	account := creationCatalogTestVideoAccount(VideoProtocolSeedance, map[string]any{
		"account-alias": "seedance-upstream", "seedance-*": "seedance-upstream",
	})
	unmapped := creationCatalogTestVideoAccount(VideoProtocolSeedance, nil)
	entry := buildCreationGroup(group, []*Account{account, unmapped}, channel, false)
	require.NotNil(t, entry.Video)
	require.Equal(t, []CreationModel{
		{ID: "account-alias", Protocols: []string{"seedance"}},
		{ID: "channel-alias", Protocols: []string{"seedance"}},
		{ID: "seedance-channel", Protocols: []string{"seedance"}},
		{ID: "seedance-duplicate", Protocols: []string{"seedance"}},
		{ID: "seedance-group", Protocols: []string{"seedance"}},
		{ID: "seedance-per-model", Protocols: []string{"seedance"}},
	}, entry.Video.Models)
}

func TestCreationCatalogImageAccountMappingProtocols(t *testing.T) {
	for _, test := range []struct {
		platform string
		upstream string
		protocol string
	}{
		{platform: PlatformOpenAI, upstream: "gpt-image-2", protocol: "openai"},
		{platform: PlatformGrok, upstream: "grok-imagine-image", protocol: "openai"},
		{platform: PlatformGemini, upstream: "gemini-2.5-flash-image", protocol: "gemini"},
	} {
		t.Run(test.platform, func(t *testing.T) {
			account := creationCatalogTestAccount(test.platform, map[string]any{"public-image": test.upstream})
			entry := buildCreationGroup(creationCatalogTestGroup(test.platform), []*Account{account}, nil, true)
			require.NotNil(t, entry.Image)
			require.Equal(t, []CreationModel{{ID: "public-image", Protocols: []string{test.protocol}}}, entry.Image.Models)
		})
	}
}

func TestCreationCatalogMappingTargetsStayPrivate(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformGrok, PlatformGemini} {
		t.Run(platform, func(t *testing.T) {
			upstream := "gpt-image-2"
			if platform == PlatformGrok {
				upstream = "grok-imagine-image"
			} else if platform == PlatformGemini {
				upstream = "gemini-2.5-flash-image"
			}
			account := creationCatalogTestAccount(platform, map[string]any{"account-public-image": upstream})
			channel := &Channel{
				Status: StatusActive,
				ModelMapping: map[string]map[string]string{platform: {
					"channel-public-image": upstream,
				}},
			}
			entry := buildCreationGroup(creationCatalogTestGroup(platform), []*Account{account}, channel, false)
			require.NotNil(t, entry.Image)
			protocol := "openai"
			if platform == PlatformGemini {
				protocol = "gemini"
			}
			require.Equal(t, []CreationModel{
				{ID: "account-public-image", Protocols: []string{protocol}},
				{ID: "channel-public-image", Protocols: []string{protocol}},
			}, entry.Image.Models)
			encoded, err := json.Marshal(entry)
			require.NoError(t, err)
			require.NotContains(t, string(encoded), upstream)
		})
	}
}

func TestCreationCatalogChannelActiveAndPlatformIsolation(t *testing.T) {
	for _, status := range []string{StatusActive, "disabled"} {
		t.Run(status, func(t *testing.T) {
			channel := &Channel{
				Status: status,
				ModelPricing: []ChannelModelPricing{
					{Platform: PlatformOpenAI, BillingMode: BillingModeImage, Models: []string{"channel-image"}},
					{Platform: PlatformOpenAI, BillingMode: BillingModeVideo, Models: []string{"seedance-channel"}},
					{Platform: PlatformGemini, BillingMode: BillingModeImage, Models: []string{"foreign-image"}},
					{Platform: PlatformVideo, BillingMode: BillingModeVideo, Models: []string{"veo-foreign"}},
				},
				ModelMapping: map[string]map[string]string{
					PlatformOpenAI: {"image-alias": "gpt-image-2", "video-alias": "seedance-channel"},
					PlatformGemini: {"foreign-alias": "gpt-image-2"},
				},
			}
			entry := buildCreationGroup(creationCatalogTestGroup(PlatformOpenAI), []*Account{creationCatalogTestSeedanceAccount(nil)}, channel, false)
			require.NotNil(t, entry.Image)
			require.NotNil(t, entry.Video)
			if status != StatusActive {
				// 停用渠道不贡献任何模型，于是回落到透传账号的平台默认生图模型。
				require.Equal(t, creationCatalogDefaultOpenAIImageModels, entry.Image.Models)
				require.Equal(t, []CreationModel{}, entry.Video.Models)
				return
			}
			require.Equal(t, []CreationModel{
				{ID: "channel-image", Protocols: []string{"openai"}},
				{ID: "image-alias", Protocols: []string{"openai"}},
			}, entry.Image.Models)
			require.Equal(t, []CreationModel{
				{ID: "seedance-channel", Protocols: []string{"seedance"}},
				{ID: "video-alias", Protocols: []string{"seedance"}},
			}, entry.Video.Models)
		})
	}
}

func TestCreationCatalogAllowlistAndWildcardCandidates(t *testing.T) {
	group := creationCatalogTestGroup(PlatformOpenAI)
	group.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"gpt-image-2", "seedance-pro", "public-*", "unpriced"}}
	channel := &Channel{
		Status: StatusActive,
		ModelPricing: []ChannelModelPricing{
			{Platform: PlatformOpenAI, BillingMode: BillingModeImage, Models: []string{"gpt-image-*", "blocked-image"}},
			{Platform: PlatformOpenAI, BillingMode: BillingModeVideo, Models: []string{"seedance-*", "blocked-video"}},
		},
	}
	account := creationCatalogTestSeedanceAccount(map[string]any{
		"public-image": "gpt-image-1", "public-video": "seedance-1-pro", "private-image": "gpt-image-1",
		"private-video": "seedance-1-pro", "public-*": "gpt-image-1",
	})
	unmapped := creationCatalogTestSeedanceAccount(nil)
	entry := buildCreationGroup(group, []*Account{account, unmapped}, channel, false)
	require.NotNil(t, entry.Image)
	require.NotNil(t, entry.Video)
	require.Equal(t, []CreationModel{
		{ID: "gpt-image-2", Protocols: []string{"openai"}},
		{ID: "public-image", Protocols: []string{"openai"}},
	}, entry.Image.Models)
	require.Equal(t, []CreationModel{
		{ID: "public-video", Protocols: []string{"seedance"}},
		{ID: "seedance-pro", Protocols: []string{"seedance"}},
	}, entry.Video.Models)
}

func TestCreationCatalogExactAllowlistExpandsAccountWildcards(t *testing.T) {
	group := creationCatalogTestGroup(PlatformOpenAI)
	group.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"public-image-one", "public-video-one", "unmatched"}}
	account := creationCatalogTestSeedanceAccount(map[string]any{
		"public-image-*": "gpt-image-2", "public-video-*": "seedance-1-pro", "hidden-image": "gpt-image-2",
	})
	entry := buildCreationGroup(group, []*Account{account}, nil, false)
	require.NotNil(t, entry.Image)
	require.NotNil(t, entry.Video)
	require.Equal(t, []CreationModel{{ID: "public-image-one", Protocols: []string{"openai"}}}, entry.Image.Models)
	require.Equal(t, []CreationModel{{ID: "public-video-one", Protocols: []string{"seedance"}}}, entry.Video.Models)
}

func TestCreationCatalogChannelRestrictModels(t *testing.T) {
	for _, restricted := range []bool{false, true} {
		name := "unrestricted"
		if restricted {
			name = "restricted"
		}
		t.Run(name, func(t *testing.T) {
			channel := &Channel{
				Status: StatusActive, RestrictModels: restricted,
				ModelPricing: []ChannelModelPricing{
					{Platform: PlatformOpenAI, BillingMode: BillingModeImage, Models: []string{"allowed-image"}},
					{Platform: PlatformOpenAI, BillingMode: BillingModeVideo, Models: []string{"allowed-video"}},
					{Platform: PlatformGemini, BillingMode: BillingModeImage, Models: []string{"blocked-image"}},
					{Platform: PlatformVideo, BillingMode: BillingModeVideo, Models: []string{"blocked-video"}},
				},
			}
			account := creationCatalogTestSeedanceAccount(map[string]any{
				"allowed-image": "gpt-image-2", "blocked-image": "gpt-image-2",
				"allowed-video": "seedance-1-pro", "blocked-video": "seedance-1-pro",
			})
			entry := buildCreationGroup(creationCatalogTestGroup(PlatformOpenAI), []*Account{account}, channel, false)
			require.NotNil(t, entry.Image)
			require.NotNil(t, entry.Video)
			wantImages := []CreationModel{{ID: "allowed-image", Protocols: []string{"openai"}}}
			wantVideos := []CreationModel{{ID: "allowed-video", Protocols: []string{"seedance"}}}
			if !restricted {
				wantImages = append(wantImages, CreationModel{ID: "blocked-image", Protocols: []string{"openai"}})
				wantVideos = append(wantVideos, CreationModel{ID: "blocked-video", Protocols: []string{"seedance"}})
			}
			require.Equal(t, wantImages, entry.Image.Models)
			require.Equal(t, wantVideos, entry.Video.Models)
		})
	}
}

func TestCreationCatalogChannelRestrictionsUsePublicWildcardPricing(t *testing.T) {
	group := creationCatalogTestGroup(PlatformOpenAI)
	group.ModelPricing = []ChannelModelPricing{{Platform: PlatformOpenAI, BillingMode: BillingModeImage, Models: []string{"group-only-image"}}}
	channel := &Channel{
		Status: StatusActive, RestrictModels: true,
		ModelPricing: []ChannelModelPricing{
			{Platform: PlatformOpenAI, BillingMode: BillingModeImage, Models: []string{"public-image-*", "gpt-image-2"}},
			{Platform: PlatformOpenAI, BillingMode: BillingModeVideo, Models: []string{"public-video-*", "seedance-1-pro"}},
		},
		ModelMapping: map[string]map[string]string{
			PlatformOpenAI: {"unpriced-image-alias": "gpt-image-2", "unpriced-video-alias": "seedance-1-pro"},
		},
	}
	account := creationCatalogTestSeedanceAccount(map[string]any{
		"public-image-one": "gpt-image-2", "public-video-one": "seedance-1-pro",
	})
	unmapped := creationCatalogTestSeedanceAccount(nil)
	entry := buildCreationGroup(group, []*Account{account, unmapped}, channel, false)
	require.NotNil(t, entry.Image)
	require.NotNil(t, entry.Video)
	require.Equal(t, []CreationModel{
		{ID: "gpt-image-2", Protocols: []string{"openai"}},
		{ID: "public-image-one", Protocols: []string{"openai"}},
	}, entry.Image.Models)
	require.Equal(t, []CreationModel{
		{ID: "public-video-one", Protocols: []string{"seedance"}},
		{ID: "seedance-1-pro", Protocols: []string{"seedance"}},
	}, entry.Video.Models)
}

func TestCreationCatalogAccountAvailability(t *testing.T) {
	future, past := time.Now().Add(24*time.Hour), time.Now().Add(-24*time.Hour)
	for _, test := range []struct {
		name   string
		mutate func(*Account)
	}{
		{name: "inactive", mutate: func(account *Account) { account.Status = "disabled" }},
		{name: "unschedulable", mutate: func(account *Account) { account.Schedulable = false }},
		{name: "rate limited", mutate: func(account *Account) { account.RateLimitResetAt = &future }},
		{name: "overloaded", mutate: func(account *Account) { account.OverloadUntil = &future }},
		{name: "temporarily unavailable", mutate: func(account *Account) { account.TempUnschedulableUntil = &future }},
		{name: "expired", mutate: func(account *Account) { account.AutoPauseOnExpired, account.ExpiresAt = true, &past }},
		{name: "foreign platform", mutate: func(account *Account) { account.Platform = PlatformGrok }},
	} {
		t.Run(test.name, func(t *testing.T) {
			account := creationCatalogTestSeedanceAccount(map[string]any{"blocked-image": "gpt-image-2", "blocked-video": "seedance-1-pro"})
			test.mutate(account)
			entry := buildCreationGroup(creationCatalogTestGroup(PlatformOpenAI), []*Account{account}, nil, false)
			require.NotNil(t, entry.Image)
			require.Equal(t, []CreationModel{}, entry.Image.Models)
			require.Nil(t, entry.Video)
		})
	}
	t.Run("video requires apikey", func(t *testing.T) {
		account := creationCatalogTestVideoAccount(VideoProtocolSeedance, map[string]any{"blocked-video": "seedance-1-pro"})
		account.Type = AccountTypeOAuth
		entry := buildCreationGroup(creationCatalogTestGroup(PlatformVideo), []*Account{account}, nil, false)
		require.Nil(t, entry.Image)
		require.Nil(t, entry.Video)
	})
}

func TestCreationCatalogVideoCapabilityEnvelope(t *testing.T) {
	seedance := creationCatalogTestVideoAccount(VideoProtocolSeedance, nil)
	seedance.Credentials["video_resolutions"] = []string{"1080P", "720p", "720p"}
	seedance.Credentials["video_min_duration_seconds"], seedance.Credentials["video_max_duration_seconds"] = 4, 8
	gemini := creationCatalogTestVideoAccount(VideoProtocolGemini, nil)
	gemini.Credentials["video_resolutions"] = []any{"480p", "720P", "2K"}
	gemini.Credentials["video_min_duration_seconds"], gemini.Credentials["video_max_duration_seconds"] = float64(2), float64(15)
	minimax := creationCatalogTestVideoAccount(VideoProtocolMiniMax, nil)
	minimax.Credentials["video_resolutions"] = []string{"768p", "2k"}
	minimax.Credentials["video_min_duration_seconds"], minimax.Credentials["video_max_duration_seconds"] = 5, 20
	ignored := creationCatalogTestVideoAccount(VideoProtocolSeedance, nil)
	ignored.Status = "disabled"
	ignored.Credentials["video_min_duration_seconds"], ignored.Credentials["video_max_duration_seconds"] = 1, 60
	entry := buildCreationGroup(creationCatalogTestGroup(PlatformVideo), []*Account{
		seedance, gemini, minimax, ignored, creationCatalogTestVideoAccount(VideoProtocolSeedance, nil),
	}, nil, false)
	require.NotNil(t, entry.Video)
	require.Equal(t, []string{"gemini", "minimax_v1", "seedance"}, entry.Video.Protocols)
	require.Equal(t, []string{"480p", "720p", "768p", "1080p", "2k"}, entry.Video.Resolutions)
	require.NotNil(t, entry.Video.MinDuration)
	require.NotNil(t, entry.Video.MaxDuration)
	require.Equal(t, 2, *entry.Video.MinDuration)
	require.Equal(t, 20, *entry.Video.MaxDuration)
}

func TestCreationCatalogEmptyModelsAndUnconfiguredVideoJSON(t *testing.T) {
	group := creationCatalogTestGroup(PlatformVideo)
	entry := buildCreationGroup(group, []*Account{creationCatalogTestVideoAccount(VideoProtocolSeedance, nil)}, nil, false)
	require.NotNil(t, entry.Video)
	encoded, err := json.Marshal(entry)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":7,"name":"创作分组","description":"图片和视频","platform":"video","subscription_type":"standard","rate_multiplier":1.25,"image":null,"video":{"protocols":["seedance"],"models":[{"id":"doubao-seedance-1-0-pro","protocols":["seedance"],"pricing":null}],"resolutions":[],"min_duration":null,"max_duration":null}}`, string(encoded))
	image := buildCreationGroup(creationCatalogTestGroup(PlatformOpenAI), nil, nil, false)
	encoded, err = json.Marshal(image.Image)
	require.NoError(t, err)
	require.JSONEq(t, `{"protocols":["openai"],"models":[],"async_supported":false,"edit_supported":true}`, string(encoded))
}

var creationCatalogDefaultOpenAIImageModels = []CreationModel{
	{ID: "gpt-image-1", Protocols: []string{"openai"}},
	{ID: "gpt-image-1.5", Protocols: []string{"openai"}},
	{ID: "gpt-image-2", Protocols: []string{"openai"}},
	{ID: "gpt-image-2.5-flare", Protocols: []string{"openai"}},
	{ID: "gpt-image-2.5-sunburst", Protocols: []string{"openai"}},
}

func TestCreationCatalogDefaultModelsForPassthroughAccounts(t *testing.T) {
	t.Run("openai image", func(t *testing.T) {
		entry := buildCreationGroup(creationCatalogTestGroup(PlatformOpenAI), []*Account{creationCatalogTestAccount(PlatformOpenAI, nil)}, nil, false)
		require.NotNil(t, entry.Image)
		require.Equal(t, creationCatalogDefaultOpenAIImageModels, entry.Image.Models)
	})
	t.Run("grok uses canonical models instead of built-in aliases", func(t *testing.T) {
		account := creationCatalogTestAccount(PlatformGrok, nil)
		require.NotEmpty(t, account.GetModelMapping(), "Grok 账号未配置映射时仍有内置别名表")
		entry := buildCreationGroup(creationCatalogTestGroup(PlatformGrok), []*Account{account}, nil, true)
		require.NotNil(t, entry.Image)
		require.True(t, entry.Image.AsyncSupported)
		require.Equal(t, []CreationModel{
			{ID: "grok-imagine-image", Protocols: []string{"openai"}},
			{ID: "grok-imagine-image-2.0", Protocols: []string{"openai"}},
			{ID: "grok-imagine-image-quality", Protocols: []string{"openai"}},
		}, entry.Image.Models)
		require.NotNil(t, entry.Video)
		require.Equal(t, []CreationModel{
			{ID: "grok-imagine-video", Protocols: []string{"grok"}},
			{ID: "grok-imagine-video-1.5", Protocols: []string{"grok"}},
		}, entry.Video.Models)
	})
	t.Run("gemini image", func(t *testing.T) {
		entry := buildCreationGroup(creationCatalogTestGroup(PlatformGemini), []*Account{creationCatalogTestAccount(PlatformGemini, nil)}, nil, false)
		require.NotNil(t, entry.Image)
		require.Equal(t, []CreationModel{
			{ID: "gemini-2.5-flash-image", Protocols: []string{"gemini"}},
			{ID: "gemini-3.1-flash-image", Protocols: []string{"gemini"}},
		}, entry.Image.Models)
	})
	for _, test := range []struct {
		protocol string
		want     []CreationModel
	}{
		{protocol: VideoProtocolSeedance, want: []CreationModel{{ID: "doubao-seedance-1-0-pro", Protocols: []string{"seedance"}}}},
		{protocol: VideoProtocolGemini, want: []CreationModel{
			{ID: "veo-3.0-fast-generate-001", Protocols: []string{"gemini"}},
			{ID: "veo-3.0-generate-001", Protocols: []string{"gemini"}},
		}},
		{protocol: VideoProtocolMiniMax, want: []CreationModel{
			{ID: "MiniMax-Hailuo-02", Protocols: []string{"minimax_v1"}},
			{ID: "MiniMax-Hailuo-2.3", Protocols: []string{"minimax_v1"}},
			{ID: "MiniMax-Hailuo-2.3-Fast", Protocols: []string{"minimax_v1"}},
		}},
	} {
		t.Run("video "+test.protocol, func(t *testing.T) {
			entry := buildCreationGroup(creationCatalogTestGroup(PlatformVideo), []*Account{creationCatalogTestVideoAccount(test.protocol, nil)}, nil, false)
			require.NotNil(t, entry.Video)
			require.Equal(t, test.want, entry.Video.Models)
		})
	}
	t.Run("configured sources win", func(t *testing.T) {
		group := creationCatalogTestGroup(PlatformOpenAI)
		group.ModelPricing = []ChannelModelPricing{{Platform: PlatformOpenAI, BillingMode: BillingModeImage, Models: []string{"gpt-image-2"}}}
		entry := buildCreationGroup(group, []*Account{creationCatalogTestAccount(PlatformOpenAI, nil)}, nil, false)
		require.Equal(t, []CreationModel{{ID: "gpt-image-2", Protocols: []string{"openai"}}}, entry.Image.Models)
	})
	t.Run("mapped accounts and allowlist stay authoritative", func(t *testing.T) {
		mapped := buildCreationGroup(creationCatalogTestGroup(PlatformOpenAI), []*Account{creationCatalogTestAccount(PlatformOpenAI, map[string]any{"plain-chat": "gpt-4.1"})}, nil, false)
		require.Equal(t, []CreationModel{}, mapped.Image.Models)
		group := creationCatalogTestGroup(PlatformOpenAI)
		group.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"gpt-4.1"}}
		restricted := buildCreationGroup(group, []*Account{creationCatalogTestAccount(PlatformOpenAI, nil)}, nil, false)
		require.Equal(t, []CreationModel{}, restricted.Image.Models)
	})
	t.Run("no accounts", func(t *testing.T) {
		entry := buildCreationGroup(creationCatalogTestGroup(PlatformOpenAI), nil, nil, false)
		require.Equal(t, []CreationModel{}, entry.Image.Models)
	})
}

func TestCreationCatalogMiniMaxVersions(t *testing.T) {
	for _, test := range []struct {
		name      string
		baseURL   string
		protocols []string
	}{
		{name: "v1", baseURL: "https://proxy.example/v1", protocols: []string{"minimax_v1"}},
		{name: "v1 endpoint", baseURL: "https://proxy.example/v1/video_generation", protocols: []string{"minimax_v1"}},
		{name: "v2", baseURL: "https://proxy.example/v2", protocols: []string{"minimax_v1", "minimax_v2"}},
		{name: "v2 trailing slash", baseURL: "https://proxy.example/minimax/v2/", protocols: []string{"minimax_v1", "minimax_v2"}},
		{name: "v2 endpoint", baseURL: "https://proxy.example/minimax/v2/video_generation", protocols: []string{"minimax_v1", "minimax_v2"}},
		{name: "version is path not host", baseURL: "https://v2.example/v1", protocols: []string{"minimax_v1"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			account := creationCatalogTestVideoAccount(VideoProtocolMiniMax, map[string]any{"public-video": "MiniMax-Hailuo-02"})
			account.Credentials["base_url"] = test.baseURL
			entry := buildCreationGroup(creationCatalogTestGroup(PlatformVideo), []*Account{account}, nil, false)
			require.NotNil(t, entry.Video)
			require.Equal(t, test.protocols, entry.Video.Protocols)
			require.Equal(t, []CreationModel{{ID: "public-video", Protocols: test.protocols}}, entry.Video.Models)
		})
	}
	t.Run("v2 accepts legacy request", func(t *testing.T) {
		account := creationCatalogTestVideoAccount(VideoProtocolMiniMax, map[string]any{"public-video": "MiniMax-H3"})
		account.Credentials["base_url"] = "https://proxy.example/v2"
		prepared, info, err := NormalizeMiniMaxSubmission([]byte(`{"model":"public-video","prompt":"waves","duration":6}`), account, false)
		require.NoError(t, err)
		require.Equal(t, "public-video", info.Model)
		require.Equal(t, "waves", info.Prompt)
		require.Equal(t, 6, info.DurationSeconds)
		require.Contains(t, string(prepared), `"content"`)
		entry := buildCreationGroup(creationCatalogTestGroup(PlatformVideo), []*Account{account}, nil, false)
		require.NotNil(t, entry.Video)
		require.Equal(t, []CreationModel{{ID: "public-video", Protocols: []string{"minimax_v1", "minimax_v2"}}}, entry.Video.Models)
	})
	for _, test := range []struct {
		name     string
		public   string
		upstream string
	}{
		{name: "H3 cannot use v1 account", public: "public-video", upstream: "MiniMax-H3"},
		{name: "H3 Max cannot use v1 account", public: "public-video", upstream: "MiniMax-H3-Max"},
		{name: "public H3 cannot use v1 account", public: "MiniMax-H3", upstream: "MiniMax-Hailuo-02"},
	} {
		t.Run(test.name, func(t *testing.T) {
			account := creationCatalogTestVideoAccount(VideoProtocolMiniMax, map[string]any{test.public: test.upstream})
			body, err := json.Marshal(map[string]any{"model": test.public, "prompt": "waves", "duration": 6})
			require.NoError(t, err)
			_, _, err = NormalizeMiniMaxSubmission(body, account, false)
			require.ErrorIs(t, err, ErrMiniMaxVideoAccountIncompatible)
			entry := buildCreationGroup(creationCatalogTestGroup(PlatformVideo), []*Account{account}, nil, false)
			if entry.Video != nil {
				require.Empty(t, entry.Video.Models)
			}
		})
	}
}

func TestCreationCatalogVideoModelProtocolOwnership(t *testing.T) {
	group := creationCatalogTestGroup(PlatformVideo)
	group.ModelPricing = []ChannelModelPricing{{
		Platform: PlatformVideo, BillingMode: BillingModeVideo,
		Models: []string{"veo-3", "seedance-1-pro", "MiniMax-H3", "hailuo-02", "unknown-model"},
	}}
	minimax := creationCatalogTestVideoAccount(VideoProtocolMiniMax, nil)
	minimax.Credentials["base_url"] = "https://proxy.example/v2"
	entry := buildCreationGroup(group, []*Account{
		creationCatalogTestVideoAccount(VideoProtocolSeedance, nil),
		creationCatalogTestVideoAccount(VideoProtocolGemini, nil), minimax,
	}, nil, false)
	require.NotNil(t, entry.Video)
	require.Equal(t, []CreationModel{
		{ID: "MiniMax-H3", Protocols: []string{"minimax_v1", "minimax_v2"}},
		{ID: "hailuo-02", Protocols: []string{"minimax_v1", "minimax_v2"}},
		{ID: "seedance-1-pro", Protocols: []string{"seedance"}},
		{ID: "unknown-model", Protocols: []string{}},
		{ID: "veo-3", Protocols: []string{"gemini"}},
	}, entry.Video.Models)
	t.Run("account aliases and protocol union", func(t *testing.T) {
		entry := buildCreationGroup(creationCatalogTestGroup(PlatformVideo), []*Account{
			creationCatalogTestVideoAccount(VideoProtocolSeedance, map[string]any{"seed-alias": "private-seed", "shared-alias": "private-seed"}),
			creationCatalogTestVideoAccount(VideoProtocolGemini, map[string]any{"gemini-alias": "private-gemini", "shared-alias": "private-gemini"}),
		}, nil, false)
		require.NotNil(t, entry.Video)
		require.Equal(t, []CreationModel{
			{ID: "gemini-alias", Protocols: []string{"gemini"}},
			{ID: "seed-alias", Protocols: []string{"seedance"}},
			{ID: "shared-alias", Protocols: []string{"gemini", "seedance"}},
		}, entry.Video.Models)
	})
}

func TestCreationCatalogGrokMediaDisabled(t *testing.T) {
	blocked := creationCatalogTestAccount(PlatformGrok, map[string]any{"blocked-image": "grok-imagine-image", "blocked-video": "grok-imagine-video"})
	blocked.Extra = map[string]any{GrokMediaEligibleExtraKey: false}
	allowed := creationCatalogTestAccount(PlatformGrok, map[string]any{"public-image": "grok-imagine-image", "public-video": "grok-imagine-video"})
	entry := buildCreationGroup(creationCatalogTestGroup(PlatformGrok), []*Account{blocked, allowed}, nil, false)
	require.NotNil(t, entry.Image)
	require.NotNil(t, entry.Video)
	require.Equal(t, []CreationModel{{ID: "public-image", Protocols: []string{"openai"}}}, entry.Image.Models)
	require.Equal(t, []CreationModel{{ID: "public-video", Protocols: []string{"grok"}}}, entry.Video.Models)
	entry = buildCreationGroup(creationCatalogTestGroup(PlatformGrok), []*Account{blocked}, nil, false)
	require.NotNil(t, entry.Image)
	require.Empty(t, entry.Image.Models)
	require.Nil(t, entry.Video)
}

func TestCreationCatalogKnownVideoFamilyMismatchExcluded(t *testing.T) {
	for _, test := range []struct {
		protocol string
		model    string
	}{
		{protocol: VideoProtocolSeedance, model: "veo-3"},
		{protocol: VideoProtocolSeedance, model: "MiniMax-H3"},
		{protocol: VideoProtocolGemini, model: "seedance-1-pro"},
		{protocol: VideoProtocolGemini, model: "grok-imagine-video"},
		{protocol: VideoProtocolMiniMax, model: "veo-3"},
	} {
		t.Run(test.protocol+"/"+test.model, func(t *testing.T) {
			group := creationCatalogTestGroup(PlatformVideo)
			group.ModelPricing = []ChannelModelPricing{{Platform: PlatformVideo, BillingMode: BillingModeVideo, Models: []string{test.model}}}
			account := creationCatalogTestVideoAccount(test.protocol, nil)
			entry := buildCreationGroup(group, []*Account{account}, nil, false)
			require.NotNil(t, entry.Video)
			require.Equal(t, []CreationModel{}, entry.Video.Models)
		})
	}
}

func TestCreationCatalogGetCatalogEmpty(t *testing.T) {
	groups := &creationCatalogGroupStub{}
	accounts := &creationCatalogAccountStub{err: errors.New("must not query accounts")}
	resolverCalls := 0
	tasks := NewImageTaskServiceWithResolver(&creationCatalogTaskStore{}, func() (*ImageResultUploader, bool) {
		resolverCalls++
		return nil, true
	}, time.Hour, time.Minute)
	service := &CreationCatalogService{groups: groups, accounts: accounts, tasks: tasks}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	catalog, err := service.GetCatalog(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, &CreationCatalog{Groups: []CreationGroup{}}, catalog)
	require.Equal(t, 1, groups.calls)
	require.Equal(t, int64(42), groups.userID)
	require.Same(t, ctx, groups.ctx)
	require.Zero(t, accounts.calls)
	require.Zero(t, resolverCalls)
	encoded, err := json.Marshal(catalog)
	require.NoError(t, err)
	require.JSONEq(t, `{"groups":[]}`, string(encoded))
}

func TestCreationCatalogGetCatalogBatchAndGroupIsolation(t *testing.T) {
	image := creationCatalogTestGroup(PlatformOpenAI)
	video := creationCatalogTestGroup(PlatformVideo)
	video.ID = 8
	unsupported := creationCatalogTestGroup(PlatformComposite)
	unsupported.ID = 9
	empty := creationCatalogTestGroup(PlatformVideo)
	empty.ID = 10
	groups := &creationCatalogGroupStub{groups: []Group{*image, *video, *unsupported, *empty}}
	imageAccount := creationCatalogTestAccount(PlatformOpenAI, map[string]any{"public-image": "gpt-image-2"})
	videoAccount := creationCatalogTestVideoAccount(VideoProtocolSeedance, map[string]any{"public-video": "seedance-1-pro"})
	videoAccount.GroupIDs = []int64{8}
	unrelated := creationCatalogTestSeedanceAccount(map[string]any{"unrelated-image": "gpt-image-2", "unrelated-video": "seedance-1-pro"})
	unrelated.GroupIDs = []int64{99}
	foreign := creationCatalogTestVideoAccount(VideoProtocolGemini, map[string]any{"foreign-video": "veo-3"})
	blocked := creationCatalogTestVideoAccount(VideoProtocolMiniMax, map[string]any{"blocked-video": "hailuo-02"})
	blocked.GroupIDs, blocked.Schedulable = []int64{8, 10}, false
	inactive := creationCatalogTestVideoAccount(VideoProtocolGemini, map[string]any{"inactive-video": "veo-3"})
	inactive.GroupIDs, inactive.Status = []int64{8, 10}, "disabled"
	accounts := &creationCatalogAccountStub{accounts: []Account{*imageAccount, *videoAccount, *unrelated, *foreign, *blocked, *inactive}}
	service := &CreationCatalogService{groups: groups, accounts: accounts}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	catalog, err := service.GetCatalog(ctx, 42)
	require.NoError(t, err)
	require.Len(t, catalog.Groups, 2)
	require.Equal(t, int64(7), catalog.Groups[0].ID)
	require.NotNil(t, catalog.Groups[0].Image)
	require.Nil(t, catalog.Groups[0].Video)
	require.Equal(t, []CreationModel{{ID: "public-image", Protocols: []string{"openai"}}}, catalog.Groups[0].Image.Models)
	require.Equal(t, int64(8), catalog.Groups[1].ID)
	require.Nil(t, catalog.Groups[1].Image)
	require.NotNil(t, catalog.Groups[1].Video)
	require.Equal(t, []CreationModel{{ID: "public-video", Protocols: []string{"seedance"}}}, catalog.Groups[1].Video.Models)
	require.Equal(t, 1, groups.calls)
	require.Equal(t, int64(42), groups.userID)
	require.Same(t, ctx, groups.ctx)
	require.Equal(t, 1, accounts.calls)
	require.Same(t, ctx, accounts.ctx)
	require.ElementsMatch(t, []string{PlatformOpenAI, PlatformGrok, PlatformGemini, PlatformVideo}, accounts.platforms)
	encoded, err := json.Marshal(catalog)
	require.NoError(t, err)
	for _, private := range []string{"private-account", "private-credential", "private-upstream", "gpt-image-2", "seedance-1-pro", "account_id", "credentials", "schedulable", "account_count"} {
		require.NotContains(t, string(encoded), private)
	}
}

func TestCreationCatalogGetCatalogChannelAndSharedAccount(t *testing.T) {
	first := creationCatalogTestGroup(PlatformOpenAI)
	second := creationCatalogTestGroup(PlatformOpenAI)
	second.ID, second.SubscriptionType = 8, "subscription"
	groups := &creationCatalogGroupStub{groups: []Group{*first, *second}}
	account := creationCatalogTestAccount(PlatformOpenAI, map[string]any{"shared-image": "gpt-image-2"})
	account.GroupIDs = []int64{7, 8}
	accounts := &creationCatalogAccountStub{accounts: []Account{*account}}
	channel := &Channel{
		Status:       StatusActive,
		ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, BillingMode: BillingModeImage, Models: []string{"channel-image"}}},
	}
	cache := newEmptyChannelCache()
	cache.loadedAt = time.Now()
	cache.channelByGroupID[7] = channel
	channels := &ChannelService{}
	channels.cache.Store(cache)
	service := &CreationCatalogService{groups: groups, accounts: accounts, channels: channels}
	catalog, err := service.GetCatalog(context.Background(), 42)
	require.NoError(t, err)
	require.Len(t, catalog.Groups, 2)
	require.Equal(t, []CreationModel{
		{ID: "channel-image", Protocols: []string{"openai"}},
		{ID: "shared-image", Protocols: []string{"openai"}},
	}, catalog.Groups[0].Image.Models)
	require.Equal(t, []CreationModel{{ID: "shared-image", Protocols: []string{"openai"}}}, catalog.Groups[1].Image.Models)
	require.Equal(t, "subscription", catalog.Groups[1].SubscriptionType)
	require.Equal(t, 1, accounts.calls)
}

func TestCreationCatalogGetCatalogNoSupportedGroups(t *testing.T) {
	groups := &creationCatalogGroupStub{groups: []Group{
		*creationCatalogTestGroup(PlatformComposite), *creationCatalogTestGroup(PlatformAnthropic),
		*creationCatalogTestGroup(PlatformAntigravity), *creationCatalogTestGroup(PlatformVideo),
	}}
	accounts := &creationCatalogAccountStub{}
	catalog, err := (&CreationCatalogService{groups: groups, accounts: accounts}).GetCatalog(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, &CreationCatalog{Groups: []CreationGroup{}}, catalog)
	require.Equal(t, 1, accounts.calls)
}

func TestCreationCatalogGetCatalogReadErrors(t *testing.T) {
	for _, source := range []string{"groups", "accounts", "rates"} {
		t.Run(source, func(t *testing.T) {
			readErr := errors.New(source + " read failed")
			groups := &creationCatalogGroupStub{groups: []Group{*creationCatalogTestGroup(PlatformOpenAI)}}
			accounts := &creationCatalogAccountStub{}
			switch source {
			case "groups":
				groups.err = readErr
			case "accounts":
				accounts.err = readErr
			default:
				groups.rateErr = readErr
			}
			catalog, err := (&CreationCatalogService{groups: groups, accounts: accounts}).GetCatalog(context.Background(), 42)
			require.ErrorIs(t, err, readErr)
			require.Nil(t, catalog)
			require.Equal(t, 1, groups.calls)
			if source == "groups" {
				require.Zero(t, accounts.calls)
			} else {
				require.Equal(t, 1, accounts.calls)
			}
		})
	}
}

func TestCreationCatalogGetCatalogAsyncGateReadOnce(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		if enabled {
			name = "enabled"
		}
		t.Run(name, func(t *testing.T) {
			openai := creationCatalogTestGroup(PlatformOpenAI)
			grok := creationCatalogTestGroup(PlatformGrok)
			grok.ID = 8
			gemini := creationCatalogTestGroup(PlatformGemini)
			gemini.ID = 9
			groups := &creationCatalogGroupStub{groups: []Group{*openai, *grok, *gemini}}
			resolverCalls := 0
			tasks := NewImageTaskServiceWithResolver(&creationCatalogTaskStore{}, func() (*ImageResultUploader, bool) {
				resolverCalls++
				return nil, enabled
			}, time.Hour, time.Minute)
			service := &CreationCatalogService{groups: groups, accounts: &creationCatalogAccountStub{}, tasks: tasks}
			catalog, err := service.GetCatalog(context.Background(), 42)
			require.NoError(t, err)
			require.Len(t, catalog.Groups, 3)
			require.Equal(t, 1, resolverCalls)
			for _, entry := range catalog.Groups {
				require.NotNil(t, entry.Image)
				require.Equal(t, enabled && entry.Platform != PlatformGemini, entry.Image.AsyncSupported)
			}
		})
	}
}

func creationCatalogPrice(value float64) *float64 { return &value }

func TestCreationCatalogImagePricing(t *testing.T) {
	channel := &Channel{
		Status: StatusActive,
		ModelPricing: []ChannelModelPricing{
			{Platform: PlatformOpenAI, BillingMode: BillingModeImage, Models: []string{"gpt-image-2"}, Intervals: []PricingInterval{
				{TierLabel: "1K", PerRequestPrice: creationCatalogPrice(0.04)},
				{TierLabel: "2K", PerRequestPrice: creationCatalogPrice(0.08)},
			}},
			{Platform: PlatformOpenAI, BillingMode: BillingModeImage, Models: []string{"gpt-image-1*"}, PerRequestPrice: creationCatalogPrice(0.02)},
			{Platform: PlatformOpenAI, BillingMode: BillingModeToken, Models: []string{"gpt-image-2.5-flare"}, OutputPrice: creationCatalogPrice(0.00004)},
		},
	}
	build := func(group *Group, rate float64) []CreationModel {
		account := creationCatalogTestAccount(PlatformOpenAI, map[string]any{
			"gpt-image-2": "gpt-image-2", "gpt-image-1.5": "gpt-image-1.5",
			"gpt-image-2.5-flare": "gpt-image-2.5-flare", "gpt-image-2.5-sunburst": "gpt-image-2.5-sunburst",
		})
		entry := buildCreationGroup(group, []*Account{account}, channel, false)
		fillCreationPricing(&entry, group, channel, rate)
		return entry.Image.Models
	}
	models := build(creationCatalogTestGroup(PlatformOpenAI), 1.5)
	pricing := make(map[string]*CreationModelPricing)
	for _, model := range models {
		pricing[model.ID] = model.Pricing
	}
	require.Len(t, pricing, 4)
	require.Equal(t, &CreationModelPricing{Unit: "image", Tiers: []CreationPriceTier{{Label: "1K", Price: 0.04 * 1.5}, {Label: "2K", Price: 0.08 * 1.5}}}, pricing["gpt-image-2"])
	require.Equal(t, &CreationModelPricing{Unit: "image", Price: creationCatalogPrice(0.02 * 1.5), Tiers: []CreationPriceTier{}}, pricing["gpt-image-1.5"], "通配符渠道定价")
	require.Nil(t, pricing["gpt-image-2.5-flare"], "按 token 计费的模型不展示图片单价")
	require.Nil(t, pricing["gpt-image-2.5-sunburst"], "未配置定价")

	t.Run("group tier prices override channel tiers", func(t *testing.T) {
		group := creationCatalogTestGroup(PlatformOpenAI)
		group.ImagePrice2K, group.ImagePrice4K = creationCatalogPrice(0.1), creationCatalogPrice(0.2)
		for _, model := range build(group, 1) {
			if model.ID == "gpt-image-2" {
				require.Equal(t, []CreationPriceTier{{Label: "1K", Price: 0.04}, {Label: "2K", Price: 0.1}, {Label: "4K", Price: 0.2}}, model.Pricing.Tiers)
			}
		}
	})
	t.Run("independent image rate replaces group rate", func(t *testing.T) {
		group := creationCatalogTestGroup(PlatformOpenAI)
		group.ImageRateIndependent, group.ImageRateMultiplier = true, 0.5
		for _, model := range build(group, 3) {
			if model.ID == "gpt-image-2" {
				require.Equal(t, []CreationPriceTier{{Label: "1K", Price: 0.02}, {Label: "2K", Price: 0.04}}, model.Pricing.Tiers)
			}
		}
	})
}

func TestCreationCatalogVideoPricing(t *testing.T) {
	group := creationCatalogTestGroup(PlatformVideo)
	group.ModelPricing = []ChannelModelPricing{
		{Platform: PlatformVideo, BillingMode: BillingModeVideo, Models: []string{"seedance-second"}, Intervals: []PricingInterval{
			{TierLabel: "720p", PerRequestPrice: creationCatalogPrice(0.1)},
			{TierLabel: "1080p", PerRequestPrice: creationCatalogPrice(0.2)},
		}},
		{Platform: PlatformVideo, BillingMode: BillingModePerRequest, Models: []string{"seedance-request"}, PerRequestPrice: creationCatalogPrice(1)},
	}
	group.VideoRateIndependent, group.VideoRateMultiplier = true, 2
	account := creationCatalogTestVideoAccount(VideoProtocolSeedance, map[string]any{"seedance-second": "seedance-second", "seedance-request": "seedance-request"})
	entry := buildCreationGroup(group, []*Account{account}, nil, false)
	fillCreationPricing(&entry, group, nil, 9)
	pricing := make(map[string]*CreationModelPricing)
	for _, model := range entry.Video.Models {
		pricing[model.ID] = model.Pricing
	}
	require.Len(t, pricing, 2)
	require.Equal(t, &CreationModelPricing{Unit: "second", Tiers: []CreationPriceTier{{Label: "720p", Price: 0.2}, {Label: "1080p", Price: 0.4}}}, pricing["seedance-second"])
	require.Equal(t, &CreationModelPricing{Unit: "request", Price: creationCatalogPrice(2), Tiers: []CreationPriceTier{}}, pricing["seedance-request"])
}

func TestCreationCatalogGetCatalogUsesUserRate(t *testing.T) {
	group := creationCatalogTestGroup(PlatformOpenAI)
	group.RateMultiplier = 2
	group.ModelPricing = []ChannelModelPricing{{Platform: PlatformOpenAI, BillingMode: BillingModeImage, Models: []string{"gpt-image-2"}, PerRequestPrice: creationCatalogPrice(0.1)}}
	account := creationCatalogTestAccount(PlatformOpenAI, map[string]any{"gpt-image-2": "gpt-image-2"})
	for _, test := range []struct {
		name  string
		rates map[int64]float64
		want  float64
	}{
		{name: "group rate", want: 0.2},
		{name: "user rate", rates: map[int64]float64{7: 0.5}, want: 0.05},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &CreationCatalogService{
				groups:   &creationCatalogGroupStub{groups: []Group{*group}, rates: test.rates},
				accounts: &creationCatalogAccountStub{accounts: []Account{*account}},
			}
			catalog, err := service.GetCatalog(context.Background(), 42)
			require.NoError(t, err)
			require.Len(t, catalog.Groups, 1)
			require.InDelta(t, test.want, *catalog.Groups[0].Image.Models[0].Pricing.Price, 1e-12)
		})
	}
}
