package service

import (
	"context"
	"errors"
	"math/rand"
	"testing"
)

func TestCheckInDrawRewardDistributionAndBounds(t *testing.T) {
	rng := rand.New(rand.NewSource(20261026))
	svc := &CheckInService{randomFloat: func() (float64, error) { return rng.Float64(), nil }}
	tiers := []CheckInTier{{Min: 0.1, Max: 0.5, Weight: 70}, {Min: 0.5, Max: 1, Weight: 25}, {Min: 1, Max: 5, Weight: 5}}
	counts := make([]int, len(tiers))
	for i := 0; i < 10000; i++ {
		idx, reward, err := svc.drawReward(tiers)
		if err != nil {
			t.Fatal(err)
		}
		counts[idx]++
		if reward < tiers[idx].Min || reward > tiers[idx].Max {
			t.Fatalf("reward %v outside tier %d [%v,%v]", reward, idx, tiers[idx].Min, tiers[idx].Max)
		}
	}
	wants := []float64{0.70, 0.25, 0.05}
	for i, want := range wants {
		got := float64(counts[i]) / 10000
		if got < want-0.03 || got > want+0.03 {
			t.Fatalf("tier %d proportion = %.4f, want %.4f ± .03 (counts=%d)", i, got, want, counts[i])
		}
	}
}

func TestCheckInDrawRewardMinEqualsMax(t *testing.T) {
	svc := &CheckInService{randomFloat: func() (float64, error) { return 0.75, nil }}
	idx, reward, err := svc.drawReward([]CheckInTier{{Min: 2.3456, Max: 2.3456, Weight: 1}})
	if err != nil || idx != 0 || reward != 2.3456 {
		t.Fatalf("drawReward() = (%d, %v, %v)", idx, reward, err)
	}
}

func TestCheckInDrawRewardRoundsToFourDecimals(t *testing.T) {
	svc := &CheckInService{randomFloat: func() (float64, error) { return 0.123456, nil }}
	_, reward, err := svc.drawReward([]CheckInTier{{Min: 0, Max: 1, Weight: 1}})
	if err != nil || reward != 0.1235 {
		t.Fatalf("drawReward() reward = %v, err = %v", reward, err)
	}
}

// R1：验证开关打开但没有任何可用服务商时，captchaRequired 为 false，签到必须放行。
func TestCheckInVerifyCaptchaSkipsWhenNoProviderConfigured(t *testing.T) {
	verifier := &recordingCaptchaVerifier{}
	svc := &CheckInService{captchaVerifier: verifier}
	if err := svc.verifyCaptchaIfRequired(context.Background(), false, CaptchaProof{}, "1.2.3.4"); err != nil {
		t.Fatalf("verifyCaptchaIfRequired() = %v, want nil", err)
	}
	if verifier.calls != 0 {
		t.Fatalf("verifier called %d times, want 0", verifier.calls)
	}
}

func TestCheckInVerifyCaptchaRequiresProofWhenEnabled(t *testing.T) {
	svc := &CheckInService{captchaVerifier: &recordingCaptchaVerifier{}}
	if err := svc.verifyCaptchaIfRequired(context.Background(), true, CaptchaProof{}, "1.2.3.4"); !errors.Is(err, ErrCheckInCaptchaRequired) {
		t.Fatalf("verifyCaptchaIfRequired() = %v, want ErrCheckInCaptchaRequired", err)
	}
}

func TestCheckInVerifyCaptchaVerifiesProofWhenEnabled(t *testing.T) {
	verifier := &recordingCaptchaVerifier{}
	svc := &CheckInService{captchaVerifier: verifier}
	if err := svc.verifyCaptchaIfRequired(context.Background(), true, CaptchaProof{TurnstileToken: "token"}, "1.2.3.4"); err != nil {
		t.Fatalf("verifyCaptchaIfRequired() = %v, want nil", err)
	}
	if verifier.calls != 1 || verifier.proof.TurnstileToken != "token" || verifier.remoteIP != "1.2.3.4" {
		t.Fatalf("unexpected verifier invocation: %+v", verifier)
	}
}

type recordingCaptchaVerifier struct {
	calls    int
	proof    CaptchaProof
	remoteIP string
	err      error
}

func (v *recordingCaptchaVerifier) VerifyCaptcha(_ context.Context, proof CaptchaProof, remoteIP string) error {
	v.calls++
	v.proof = proof
	v.remoteIP = remoteIP
	return v.err
}
