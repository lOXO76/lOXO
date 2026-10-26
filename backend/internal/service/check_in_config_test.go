package service

import (
	"math"
	"testing"
)

func TestNormalizeCheckInTiersBoundaries(t *testing.T) {
	valid := []CheckInTier{{Min: 0, Max: 100000, Weight: 1}}
	tests := []struct {
		name    string
		tiers   []CheckInTier
		enabled bool
		wantErr bool
	}{
		{name: "empty disabled", tiers: nil},
		{name: "empty enabled", tiers: nil, enabled: true, wantErr: true},
		{name: "negative min", tiers: []CheckInTier{{Min: -0.1, Max: 1, Weight: 1}}, wantErr: true},
		{name: "min greater than max", tiers: []CheckInTier{{Min: 2, Max: 1, Weight: 1}}, wantErr: true},
		{name: "weight zero", tiers: []CheckInTier{{Min: 0, Max: 1, Weight: 0}}, wantErr: true},
		{name: "weight negative", tiers: []CheckInTier{{Min: 0, Max: 1, Weight: -1}}, wantErr: true},
		{name: "nan", tiers: []CheckInTier{{Min: math.NaN(), Max: 1, Weight: 1}}, wantErr: true},
		{name: "infinity", tiers: []CheckInTier{{Min: 0, Max: math.Inf(1), Weight: 1}}, wantErr: true},
		{name: "too many", tiers: make([]CheckInTier, maxCheckInTiers+1), wantErr: true},
		{name: "valid", tiers: valid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateCheckInSettings(tt.enabled, tt.tiers)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateCheckInSettings() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got == nil {
				t.Fatal("expected non-nil normalized tiers")
			}
		})
	}
}

func TestParseCheckInTiersDropsInvalidItems(t *testing.T) {
	raw := `[{"min":0,"max":1,"weight":1},{"min":-1,"max":1,"weight":1},{"min":1,"max":2,"weight":2}]`
	got := parseCheckInTiers(raw)
	if len(got) != 2 || got[0].Min != 0 || got[1].Weight != 2 {
		t.Fatalf("parseCheckInTiers() = %#v", got)
	}
}

func TestCheckInRewardRange(t *testing.T) {
	minReward, maxReward := checkInRewardRange([]CheckInTier{{Min: 0.5, Max: 1, Weight: 25}, {Min: 0.1, Max: 0.5, Weight: 70}, {Min: 1, Max: 5, Weight: 5}})
	if minReward != 0.1 || maxReward != 5 {
		t.Fatalf("range = (%v, %v), want (0.1, 5)", minReward, maxReward)
	}
	if minReward, maxReward := checkInRewardRange(nil); minReward != 0 || maxReward != 0 {
		t.Fatalf("empty range = (%v, %v), want (0, 0)", minReward, maxReward)
	}
}
