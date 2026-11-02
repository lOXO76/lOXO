package service

import (
	"context"
	"math"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

type CreationCatalog struct {
	Groups []CreationGroup `json:"groups"`
}

type CreationGroup struct {
	ID               int64                    `json:"id"`
	Name             string                   `json:"name"`
	Description      string                   `json:"description"`
	Platform         string                   `json:"platform"`
	SubscriptionType string                   `json:"subscription_type"`
	RateMultiplier   float64                  `json:"rate_multiplier"`
	Image            *CreationImageCapability `json:"image"`
	Video            *CreationVideoCapability `json:"video"`
}

type CreationModel struct {
	ID        string   `json:"id"`
	Protocols []string `json:"protocols"`
	// Pricing 实付口径的展示价，未配置图片／视频定价时为 null。
	Pricing *CreationModelPricing `json:"pricing"`
}

// CreationModelPricing 与模型广场「实付」列同口径：档位价取分组价卡、渠道定价，
// 再乘生效倍率（独立生图／视频倍率优先，其次用户专属倍率，最后分组倍率）。
type CreationModelPricing struct {
	// Unit 计价单位：image 按张、second 按秒、request 按次。
	Unit string `json:"unit"`
	// Price 不分档时的单价（按次计费）。
	Price *float64 `json:"price,omitempty"`
	// Tiers 分档单价：图片按 1K／2K／4K，视频按分辨率。
	Tiers []CreationPriceTier `json:"tiers"`
}

type CreationPriceTier struct {
	Label string  `json:"label"`
	Price float64 `json:"price"`
}

type CreationImageCapability struct {
	Protocols      []string        `json:"protocols"`
	Models         []CreationModel `json:"models"`
	AsyncSupported bool            `json:"async_supported"`
	EditSupported  bool            `json:"edit_supported"`
}

type CreationVideoCapability struct {
	Protocols   []string        `json:"protocols"`
	Models      []CreationModel `json:"models"`
	Resolutions []string        `json:"resolutions"`
	MinDuration *int            `json:"min_duration"`
	MaxDuration *int            `json:"max_duration"`
}

type creationGroupSource interface {
	GetAvailableGroups(context.Context, int64) ([]Group, error)
	GetUserGroupRates(context.Context, int64) (map[int64]float64, error)
}

type creationAccountSource interface {
	ListSchedulableByPlatforms(context.Context, []string) ([]Account, error)
}

type CreationCatalogService struct {
	groups   creationGroupSource
	accounts creationAccountSource
	channels *ChannelService
	tasks    *ImageTaskService
}

func NewCreationCatalogService(groups *APIKeyService, accounts AccountRepository, channels *ChannelService, tasks *ImageTaskService) *CreationCatalogService {
	return &CreationCatalogService{groups: groups, accounts: accounts, channels: channels, tasks: tasks}
}

func (s *CreationCatalogService) GetCatalog(ctx context.Context, userID int64) (*CreationCatalog, error) {
	groups, err := s.groups.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	catalog := &CreationCatalog{Groups: make([]CreationGroup, 0, len(groups))}
	if len(groups) == 0 {
		return catalog, nil
	}
	accounts, err := s.accounts.ListSchedulableByPlatforms(ctx, []string{PlatformOpenAI, PlatformGrok, PlatformGemini, PlatformVideo})
	if err != nil {
		return nil, err
	}
	accountsByGroup := make(map[int64][]*Account)
	for index := range accounts {
		account := &accounts[index]
		if !account.IsSchedulable() {
			continue
		}
		for _, groupID := range account.GroupIDs {
			accountsByGroup[groupID] = append(accountsByGroup[groupID], account)
		}
	}
	userRates, err := s.groups.GetUserGroupRates(ctx, userID)
	if err != nil {
		return nil, err
	}
	asyncEnabled := s.tasks.Enabled()
	for index := range groups {
		group := &groups[index]
		var channel *Channel
		if s.channels != nil {
			channel, _ = s.channels.GetChannelForGroup(ctx, group.ID)
		}
		entry := buildCreationGroup(group, accountsByGroup[group.ID], channel, asyncEnabled)
		if entry.Image == nil && entry.Video == nil {
			continue
		}
		rate := group.RateMultiplier
		if userRate, ok := userRates[group.ID]; ok {
			rate = userRate
		}
		fillCreationPricing(&entry, group, channel, rate)
		catalog.Groups = append(catalog.Groups, entry)
	}
	return catalog, nil
}

// fillCreationPricing 为目录中的模型附上实付展示价。档位合成复用模型广场的 plazaDisplayPricing，
// 与模型广场展示一致；rate 为用户在该分组的生效倍率（专属倍率优先）。
func fillCreationPricing(entry *CreationGroup, group *Group, channel *Channel, rate float64) {
	lookup := creationChannelLookup(group, channel)
	pricingFor := func(model string) *ChannelModelPricing {
		if pricing := matchGroupModelPricing(group, model); pricing != nil {
			return pricing
		}
		if lookup == nil {
			return nil
		}
		pricing := lookupPricingAcrossPlatforms(lookup.cache, group.ID, lookup.platform, strings.ToLower(model))
		if pricing == nil {
			return nil
		}
		cp := pricing.Clone()
		return &cp
	}
	if entry.Image != nil {
		imageRate := rate
		if group.ImageRateIndependent {
			imageRate = group.ImageRateMultiplier
		}
		for index := range entry.Image.Models {
			model := &entry.Image.Models[index]
			if pricing := pricingFor(model.ID); pricing != nil && pricing.BillingMode == BillingModeImage {
				model.Pricing = creationPricing(plazaImageDisplayPricing(pricing, group), "image", imageRate)
			}
		}
	}
	if entry.Video != nil {
		videoRate := rate
		if group.VideoRateIndependent {
			videoRate = math.Max(0, group.VideoRateMultiplier)
		}
		for index := range entry.Video.Models {
			model := &entry.Video.Models[index]
			display := plazaVideoDisplayPricing(pricingFor(model.ID), group, model.ID)
			switch {
			case display == nil:
			case display.BillingMode == BillingModeVideo:
				model.Pricing = creationPricing(display, "second", videoRate)
			case display.BillingMode == BillingModePerRequest:
				model.Pricing = creationPricing(display, "request", videoRate)
			}
		}
	}
}

func creationPricing(display *ChannelModelPricing, unit string, rate float64) *CreationModelPricing {
	if display == nil {
		return nil
	}
	pricing := &CreationModelPricing{Unit: unit, Tiers: []CreationPriceTier{}}
	for _, interval := range display.Intervals {
		if interval.PerRequestPrice != nil && interval.TierLabel != "" {
			pricing.Tiers = append(pricing.Tiers, CreationPriceTier{Label: interval.TierLabel, Price: *interval.PerRequestPrice * rate})
		}
	}
	if len(pricing.Tiers) == 0 && display.PerRequestPrice != nil {
		price := *display.PerRequestPrice * rate
		pricing.Price = &price
	}
	if len(pricing.Tiers) == 0 && pricing.Price == nil {
		return nil
	}
	return pricing
}

func buildCreationGroup(group *Group, accounts []*Account, channel *Channel, asyncEnabled bool) CreationGroup {
	entry := CreationGroup{
		ID: group.ID, Name: group.Name, Description: group.Description,
		Platform: group.Platform, SubscriptionType: group.SubscriptionType, RateMultiplier: group.RateMultiplier,
	}
	switch group.Platform {
	case PlatformOpenAI, PlatformGrok, PlatformGemini, PlatformVideo:
	default:
		return entry
	}
	if GroupAllowsImageGeneration(group) && group.Platform != PlatformVideo {
		protocol := "openai"
		if group.Platform == PlatformGemini {
			protocol = "gemini"
		}
		entry.Image = &CreationImageCapability{
			Protocols: []string{protocol}, Models: []CreationModel{}, EditSupported: true,
			AsyncSupported: asyncEnabled && protocol == "openai",
		}
	}
	imageModels := make(map[string]map[string]bool)
	videoModels := make(map[string]map[string]bool)
	videoProtocols := make(map[string]bool)
	resolutions := make(map[string]bool)
	video := &CreationVideoCapability{Models: []CreationModel{}, Resolutions: []string{}}
	videoAccounts := make(map[*Account][]string)
	var imageAccounts []*Account
	channelLookup := creationChannelLookup(group, channel)
	for _, account := range accounts {
		if account.Platform != group.Platform || !account.IsSchedulable() || group.RequirePrivacySet && !account.IsPrivacySet() {
			continue
		}
		protocols := creationVideoProtocols(account)
		if group.Platform == PlatformGrok && !GroupAllowsImageGeneration(group) {
			protocols = nil
		}
		if len(protocols) > 0 {
			videoAccounts[account] = protocols
			for _, protocol := range protocols {
				videoProtocols[protocol] = true
			}
			if caps, err := readVideoAccountCapabilities(account); err == nil {
				for _, resolution := range caps.resolutions {
					resolutions[resolution] = true
				}
				if caps.minDuration > 0 && (video.MinDuration == nil || caps.minDuration < *video.MinDuration) {
					video.MinDuration = &caps.minDuration
				}
				if caps.maxDuration > 0 && (video.MaxDuration == nil || caps.maxDuration > *video.MaxDuration) {
					video.MaxDuration = &caps.maxDuration
				}
			}
		}
		imageAccount := entry.Image != nil
		if account.Platform == PlatformGrok {
			imageAccount = imageAccount && account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityGrokMediaGeneration)
		}
		if account.Platform == PlatformOpenAI {
			imageAccount = imageAccount && account.SupportsOpenAIImageCapability(OpenAIImagesCapabilityBasic)
		}
		if imageAccount {
			imageAccounts = append(imageAccounts, account)
		}
		for publicModel, upstreamModel := range creationExplicitModelMapping(account) {
			if imageAccount && (creationImageModel(upstreamModel) || creationImageModel(publicModel)) {
				addCreationModel(imageModels, publicModel, entry.Image.Protocols)
			}
			if len(protocols) > 0 && (account.Platform == PlatformVideo || creationVideoModel(upstreamModel) || creationVideoModel(publicModel)) {
				addCreationModel(videoModels, publicModel, protocols)
			}
		}
		for _, model := range group.ModelAllowlist.Models {
			mapped, matched := account.ResolveMappedModel(creationMappedModel(channelLookup, group.ID, model))
			if !matched {
				continue
			}
			if imageAccount && creationImageModel(mapped) {
				addCreationModel(imageModels, model, entry.Image.Protocols)
			}
			if len(protocols) > 0 && (account.Platform == PlatformVideo || creationVideoModel(mapped)) {
				addCreationModel(videoModels, model, nil)
			}
		}
	}
	pricing := append([]ChannelModelPricing(nil), group.ModelPricing...)
	if channel != nil && channel.IsActive() {
		pricing = append(pricing, channel.ModelPricing...)
	}
	for _, price := range pricing {
		if !isPlatformPricingMatch(group.Platform, price.Platform) {
			continue
		}
		for _, model := range price.Models {
			switch price.BillingMode {
			case BillingModeImage:
				if entry.Image != nil {
					addCreationModel(imageModels, model, entry.Image.Protocols)
				}
			case BillingModeVideo:
				addCreationModel(videoModels, model, nil)
			}
			if prefix, wildcard := splitWildcardSuffix(model); wildcard {
				for _, allowed := range group.ModelAllowlist.Models {
					if !strings.HasPrefix(strings.ToLower(allowed), strings.ToLower(prefix)) {
						continue
					}
					if price.BillingMode == BillingModeImage && entry.Image != nil {
						addCreationModel(imageModels, allowed, entry.Image.Protocols)
					} else if price.BillingMode == BillingModeVideo {
						addCreationModel(videoModels, allowed, nil)
					}
				}
			}
		}
	}
	for model := range group.VideoModelPrices {
		addCreationModel(videoModels, model, nil)
	}
	if channel != nil && channel.IsActive() {
		for publicModel, mappedModel := range channel.ModelMapping[group.Platform] {
			if entry.Image != nil && (imageModels[mappedModel] != nil || creationImageModel(mappedModel)) {
				addCreationModel(imageModels, publicModel, entry.Image.Protocols)
			}
			if videoModels[mappedModel] != nil || creationVideoModel(mappedModel) {
				addCreationModel(videoModels, publicModel, nil)
			}
		}
	}
	// 映射、白名单、定价与渠道都没有给出模型时，用透传账号的平台默认模型兜底（与 /v1/models 一致），
	// 避免下拉框为空；已有配置来源时保持管理员配置的列表不变。
	if entry.Image != nil && len(imageModels) == 0 {
		for _, account := range imageAccounts {
			for _, model := range creationDefaultModels(account) {
				if creationImageModel(model) {
					addCreationModel(imageModels, model, entry.Image.Protocols)
				}
			}
		}
	}
	if len(videoModels) == 0 {
		for account, protocols := range videoAccounts {
			for _, model := range creationDefaultModels(account) {
				if account.Platform == PlatformVideo || creationVideoModel(model) {
					addCreationModel(videoModels, model, protocols)
				}
			}
		}
	}
	for model := range videoModels {
		mappedModel := creationMappedModel(channelLookup, group.ID, model)
		videoModels[model] = make(map[string]bool)
		knownModel := creationKnownVideoModel(mappedModel)
		for account, protocols := range videoAccounts {
			knownModel = knownModel || creationKnownVideoModel(account.GetMappedModel(mappedModel))
			addCreationModel(videoModels, model, creationVideoModelProtocols(account, protocols, mappedModel))
		}
		if knownModel && len(videoModels[model]) == 0 {
			delete(videoModels, model)
		}
	}
	if entry.Image != nil {
		entry.Image.Models = creationModels(group, imageModels)
		entry.Image.Models = filterCreationChannelModels(group.ID, channelLookup, entry.Image.Models)
	}
	if len(videoProtocols) > 0 {
		video.Protocols = sortedCreationKeys(videoProtocols)
		video.Models = creationModels(group, videoModels)
		video.Models = filterCreationChannelModels(group.ID, channelLookup, video.Models)
		for _, resolution := range videoCapabilityResolutionOrder {
			if resolutions[resolution] {
				video.Resolutions = append(video.Resolutions, resolution)
			}
		}
		entry.Video = video
	}
	return entry
}

func creationVideoProtocols(account *Account) []string {
	switch account.Platform {
	case PlatformGrok:
		if account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityGrokMediaGeneration) {
			return []string{"grok"}
		}
	case PlatformOpenAI:
		if account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilitySeedance) {
			return []string{"seedance"}
		}
	case PlatformVideo:
		if account.Type != AccountTypeAPIKey {
			return nil
		}
		switch account.VideoProtocol() {
		case VideoProtocolSeedance, VideoProtocolGemini:
			return []string{account.VideoProtocol()}
		case VideoProtocolMiniMax:
			if miniMaxVideoV2(account.VideoBaseURL()) {
				return []string{"minimax_v1", "minimax_v2"}
			}
			return []string{"minimax_v1"}
		}
	}
	return nil
}

// creationExplicitModelMapping 只返回账号显式配置的模型映射。未配置时 GetModelMapping 可能给出平台内置的
// 别名表（如 Grok 带 grok/、xai/ 前缀与旧名的整套 xAI 别名），不适合直接列进下拉，交给 creationDefaultModels 兜底。
func creationExplicitModelMapping(account *Account) map[string]string {
	if !creationHasExplicitModelMapping(account) {
		return nil
	}
	return account.GetModelMapping()
}

func creationHasExplicitModelMapping(account *Account) bool {
	raw, _ := account.Credentials["model_mapping"].(map[string]any)
	return len(raw) > 0
}

// creationDefaultModels 返回未配置模型映射（全部透传）的账号所在平台的默认模型，与 /v1/models 的回落列表同源；
// 配置了映射的账号只认映射，返回空。
func creationDefaultModels(account *Account) []string {
	if creationHasExplicitModelMapping(account) {
		return nil
	}
	switch account.Platform {
	case PlatformOpenAI:
		return openai.DefaultModelIDs()
	case PlatformGrok:
		return xai.DefaultModelIDs()
	case PlatformGemini:
		models := make([]string, 0, len(geminicli.DefaultModels))
		for _, model := range geminicli.DefaultModels {
			models = append(models, model.ID)
		}
		return models
	case PlatformVideo:
		return DefaultVideoModelIDs()
	}
	return nil
}

func creationImageModel(model string) bool {
	return isOpenAIImageGenerationModel(model) || isGeminiImageOutputModel(model)
}

func creationChannelLookup(group *Group, channel *Channel) *channelLookup {
	if channel == nil || !channel.IsActive() {
		return nil
	}
	copyChannel := *channel
	copyChannel.GroupIDs = []int64{group.ID}
	cache := populateChannelCache([]Channel{copyChannel}, map[int64]string{group.ID: group.Platform})
	return &channelLookup{cache: cache, channel: &copyChannel, platform: group.Platform}
}

func creationMappedModel(lookup *channelLookup, groupID int64, model string) string {
	if lookup == nil {
		return model
	}
	return resolveMapping(lookup, groupID, model).MappedModel
}

func filterCreationChannelModels(groupID int64, lookup *channelLookup, models []CreationModel) []CreationModel {
	if lookup == nil {
		return models
	}
	filtered := make([]CreationModel, 0, len(models))
	for _, model := range models {
		if !checkRestricted(lookup, groupID, model.ID) {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

func creationVideoModelProtocols(account *Account, protocols []string, model string) []string {
	if !account.IsModelSupported(model) {
		return nil
	}
	if account.Platform == PlatformVideo && account.VideoProtocol() == VideoProtocolMiniMax && !miniMaxVideoV2(account.VideoBaseURL()) && miniMaxH3Model(model) {
		return nil
	}
	upstreamModel, mapped := account.ResolveMappedModel(model)
	upstreamModel = strings.ToLower(xai.StripGrokProviderPrefix(strings.TrimSpace(upstreamModel)))
	switch {
	case strings.HasPrefix(upstreamModel, "veo"):
		if account.Platform != PlatformVideo || account.VideoProtocol() != VideoProtocolGemini {
			return nil
		}
	case strings.Contains(upstreamModel, "seedance"):
		if account.Platform == PlatformVideo && account.VideoProtocol() != VideoProtocolSeedance || account.Platform == PlatformGrok {
			return nil
		}
	case strings.HasPrefix(upstreamModel, "minimax"), strings.HasPrefix(upstreamModel, "hailuo"):
		if account.Platform != PlatformVideo || account.VideoProtocol() != VideoProtocolMiniMax || miniMaxH3Model(upstreamModel) && !miniMaxVideoV2(account.VideoBaseURL()) {
			return nil
		}
	case strings.HasPrefix(upstreamModel, "grok-"), strings.HasPrefix(upstreamModel, "imagine"):
		if account.Platform != PlatformGrok {
			return nil
		}
	default:
		if !mapped {
			return nil
		}
	}
	return protocols
}

func creationVideoModel(model string) bool {
	model = strings.ToLower(xai.StripGrokProviderPrefix(strings.TrimSpace(model)))
	return strings.Contains(model, "video") || strings.HasPrefix(model, "veo") ||
		strings.Contains(model, "seedance") || strings.HasPrefix(model, "minimax-h") || strings.HasPrefix(model, "hailuo")
}

func creationKnownVideoModel(model string) bool {
	model = strings.ToLower(xai.StripGrokProviderPrefix(strings.TrimSpace(model)))
	return strings.HasPrefix(model, "veo") || strings.Contains(model, "seedance") ||
		strings.HasPrefix(model, "minimax") || strings.HasPrefix(model, "hailuo") ||
		strings.HasPrefix(model, "grok-") || strings.HasPrefix(model, "imagine")
}

func addCreationModel(models map[string]map[string]bool, model string, protocols []string) {
	model = strings.TrimSpace(model)
	if model == "" || strings.ContainsAny(model, "*?[]") {
		return
	}
	if models[model] == nil {
		models[model] = make(map[string]bool)
	}
	for _, protocol := range protocols {
		models[model][protocol] = true
	}
}

func creationModels(group *Group, candidates map[string]map[string]bool) []CreationModel {
	models := make([]CreationModel, 0, len(candidates))
	for model, protocols := range candidates {
		if group.ModelAllowlist.Allows(model) {
			models = append(models, CreationModel{ID: model, Protocols: sortedCreationKeys(protocols)})
		}
	}
	sort.Slice(models, func(first, second int) bool { return models[first].ID < models[second].ID })
	seen := make(map[string]int)
	deduped := make([]CreationModel, 0, len(models))
	for _, model := range models {
		key := strings.ToLower(model.ID)
		if index, exists := seen[key]; exists {
			protocols := make(map[string]bool)
			for _, protocol := range append(deduped[index].Protocols, model.Protocols...) {
				protocols[protocol] = true
			}
			deduped[index].Protocols = sortedCreationKeys(protocols)
			continue
		}
		seen[key] = len(deduped)
		deduped = append(deduped, model)
	}
	return deduped
}

func sortedCreationKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return keys
}
