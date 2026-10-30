package service

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

func compileProbeRegex(pattern, flags string) (*regexp.Regexp, error) {
	if len([]rune(pattern)) > 2000 {
		return nil, fmt.Errorf("regular expression exceeds 2000 characters")
	}
	if len(flags) > 8 {
		return nil, fmt.Errorf("regular expression flags exceed 8 characters")
	}
	for _, flag := range flags {
		switch flag {
		case 'i', 'm', 's':
		default:
			return nil, fmt.Errorf("invalid regular expression flag %q: only i, m, and s are allowed", flag)
		}
	}
	if flags != "" {
		pattern = "(?" + flags + ")" + pattern
	}
	return regexp.Compile(pattern)
}

func judgeProbeAnswer(re *regexp.Regexp, negate bool, answer string) bool {
	if re == nil {
		return false
	}
	matched := re.MatchString(answer)
	if negate {
		return !matched
	}
	return matched
}

func validateDegradationProbe(params DegradationProbeCreateParams) error {
	name := strings.TrimSpace(params.Name)
	if name == "" || len([]rune(name)) > 100 {
		return fmt.Errorf("name must be non-empty and at most 100 characters")
	}
	if strings.TrimSpace(params.Prompt) == "" || len([]rune(params.Prompt)) > 20000 {
		return fmt.Errorf("prompt must be non-empty and at most 20000 characters")
	}
	if params.ProbeType != "" && params.ProbeType != DegradationProbeTypeRegex && params.ProbeType != DegradationProbeTypeVisual {
		return fmt.Errorf("probe_type must be regex or visual")
	}
	if params.MaxOutputTokens != 0 && (params.MaxOutputTokens < 256 || params.MaxOutputTokens > 128000) {
		return fmt.Errorf("max_output_tokens must be 0 or between 256 and 128000")
	}
	if params.TimeoutSeconds != 0 && (params.TimeoutSeconds < 30 || params.TimeoutSeconds > 900) {
		return fmt.Errorf("timeout_seconds must be 0 or between 30 and 900")
	}
	if params.RunIntervalSeconds != 0 && (params.RunIntervalSeconds < 300 || params.RunIntervalSeconds > 604800) {
		return fmt.Errorf("run_interval_seconds must be 0 or between 300 and 604800")
	}
	if params.ProbeType == DegradationProbeTypeVisual {
		return nil
	}
	if len([]rune(params.ExpectRegex)) > 2000 {
		return fmt.Errorf("expect_regex must be at most 2000 characters")
	}
	if _, err := compileProbeRegex(params.ExpectRegex, params.RegexFlags); err != nil {
		return err
	}
	return nil
}

func normalizeDegradationProbeParams(params DegradationProbeCreateParams) DegradationProbeCreateParams {
	params.Name = strings.TrimSpace(params.Name)
	if params.ProbeType == "" {
		params.ProbeType = DegradationProbeTypeRegex
	}
	if params.ProbeType == DegradationProbeTypeVisual {
		params.ExpectRegex, params.RegexFlags, params.Negate = "", "", false
		if params.RunIntervalSeconds == 0 {
			params.RunIntervalSeconds = 86400
		}
	}
	return params
}

func degradationProbeType(value string) string {
	if value == "" {
		return DegradationProbeTypeRegex
	}
	return value
}

func degradationExecOptions(probe *DegradationProbe) DegradationProbeExecOptions {
	visual := degradationProbeType(probe.ProbeType) == DegradationProbeTypeVisual
	timeout, maxTokens := probe.TimeoutSeconds, probe.MaxOutputTokens
	if timeout == 0 {
		timeout = 90
		if visual {
			timeout = 300
		}
	}
	if maxTokens == 0 && visual {
		maxTokens = 8192
	}
	return DegradationProbeExecOptions{Timeout: time.Duration(timeout) * time.Second, Stream: visual, MaxOutputTokens: maxTokens}
}
