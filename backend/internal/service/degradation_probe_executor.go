package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type DegradationProbeExecutor struct {
	gateway       *GatewayService
	accountTests  *AccountTestService
	defaultOption DegradationProbeExecOptions
}

func NewDegradationProbeExecutor(gateway *GatewayService, accountTests *AccountTestService) *DegradationProbeExecutor {
	return &DegradationProbeExecutor{
		gateway:       gateway,
		accountTests:  accountTests,
		defaultOption: DegradationProbeExecOptions{Timeout: 90 * time.Second, MaxAnswerBytes: degradationMaxAnswerRunes},
	}
}

func (e *DegradationProbeExecutor) Execute(ctx context.Context, groupID int64, model, prompt string, opts DegradationProbeExecOptions) DegradationProbeExecResult {
	started := time.Now()
	result := DegradationProbeExecResult{}
	if e == nil || e.gateway == nil || e.accountTests == nil {
		result.Err = fmt.Errorf("degradation probe executor is unavailable")
		result.LatencyMs = time.Since(started).Milliseconds()
		return result
	}
	if opts.Timeout <= 0 {
		opts.Timeout = e.defaultOption.Timeout
	}
	if opts.MaxAnswerBytes <= 0 {
		opts.MaxAnswerBytes = e.defaultOption.MaxAnswerBytes
	}
	callCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	group, _, err := e.gateway.resolveGatewayGroup(callCtx, &groupID)
	if err != nil {
		result.Err, result.LatencyMs = err, time.Since(started).Milliseconds()
		return result
	}
	upstreamModel := model
	if group != nil && group.Platform == PlatformComposite {
		decision, matched, routeErr := e.gateway.resolveCompositeRouteDecision(callCtx, group, model, CompositeRouteEndpointAny)
		if routeErr != nil || !matched {
			if routeErr == nil {
				routeErr = fmt.Errorf("%w supporting model: %s", ErrNoAvailableAccounts, model)
			}
			result.Err, result.LatencyMs = routeErr, time.Since(started).Milliseconds()
			return result
		}
		// Keep account selection and the actual request on the same resolved route.
		callCtx = WithCompositeRouteDecision(callCtx, decision)
		upstreamModel = decision.UpstreamModel
	}
	account, err := e.gateway.SelectAccountForModelWithExclusions(callCtx, &groupID, "", model, nil)
	if err != nil {
		result.Err = err
		result.LatencyMs = time.Since(started).Milliseconds()
		return result
	}
	result.AccountID = account.ID
	probe, err := e.accountTests.RunPromptBackgroundWithOptions(callCtx, account.ID, upstreamModel, prompt, PromptProbeOptions{Timeout: opts.Timeout, Stream: opts.Stream, MaxOutputTokens: opts.MaxOutputTokens})
	if err != nil {
		result.Err = err
	} else if probe == nil {
		result.Err = fmt.Errorf("empty probe result")
	} else {
		result.fullAnswer = probe.ResponseText
		result.AnswerText = truncateDegradationRunes(probe.ResponseText, opts.MaxAnswerBytes)
		if probe.ErrorMessage != "" {
			result.Err = fmt.Errorf("%s", probe.ErrorMessage)
		} else if strings.TrimSpace(probe.ResponseText) == "" {
			result.Err = fmt.Errorf("upstream returned an empty answer")
		}
	}
	result.LatencyMs = time.Since(started).Milliseconds()
	return result
}

func truncateDegradationRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) > max {
		return string(runes[:max])
	}
	return value
}
