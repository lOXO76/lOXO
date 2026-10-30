//go:build unit

package service

import (
	"context"
	"net/http"
	"regexp"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestDegradationCompileProbeRegexFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		flags   string
		answer  string
		matched bool
	}{
		{name: "case sensitive by default", pattern: "^answer$", answer: "ANSWER"},
		{name: "ignore case", pattern: "^answer$", flags: "i", answer: "ANSWER", matched: true},
		{name: "multiline", pattern: "^answer$", flags: "m", answer: "before\nanswer\nafter", matched: true},
		{name: "dot matches newline", pattern: "^first.last$", flags: "s", answer: "first\nlast", matched: true},
		{name: "combined flags", pattern: "^answer.end$", flags: "ims", answer: "before\nANSWER\nEND\nafter", matched: true},
		{name: "reordered duplicate flags", pattern: "^answer.end$", flags: "smiims", answer: "before\nANSWER\nEND\nafter", matched: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			re, err := compileProbeRegex(tt.pattern, tt.flags)
			require.NoError(t, err)
			require.Equal(t, tt.matched, re.MatchString(tt.answer))
		})
	}

	for _, flags := range []string{"x", "I", "imx", "i m"} {
		t.Run("invalid flags "+flags, func(t *testing.T) {
			t.Parallel()
			re, err := compileProbeRegex("answer", flags)
			require.ErrorContains(t, err, "only i, m, and s are allowed")
			require.Nil(t, re)
		})
	}
}

func TestDegradationJudgeProbeAnswerNegate(t *testing.T) {
	t.Parallel()

	re := regexp.MustCompile("^correct$")
	for _, tt := range []struct {
		name   string
		negate bool
		answer string
		passed bool
	}{
		{name: "matching answer passes", answer: "correct", passed: true},
		{name: "nonmatching answer fails", answer: "incorrect"},
		{name: "matching negated answer fails", negate: true, answer: "correct"},
		{name: "nonmatching negated answer passes", negate: true, answer: "incorrect", passed: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.passed, judgeProbeAnswer(re, tt.negate, tt.answer))
		})
	}
	require.False(t, judgeProbeAnswer(nil, false, "correct"))
	require.False(t, judgeProbeAnswer(nil, true, "incorrect"))
}

func TestDegradationInvalidRegexPreservesCompilerError(t *testing.T) {
	t.Parallel()

	for _, flags := range []string{"", "im"} {
		t.Run("flags="+flags, func(t *testing.T) {
			t.Parallel()
			pattern := "["
			compiledPattern := pattern
			if flags != "" {
				compiledPattern = "(?" + flags + ")" + pattern
			}
			_, compilerErr := regexp.Compile(compiledPattern)
			require.Error(t, compilerErr)

			re, err := compileProbeRegex(pattern, flags)
			require.Nil(t, re)
			require.EqualError(t, err, compilerErr.Error())

			svc := &DegradationService{}
			matched, passed, err := svc.TestRegex(context.Background(), pattern, flags, false, "sample")
			require.False(t, matched)
			require.False(t, passed)
			require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
			require.Equal(t, "DEGRADATION_INVALID_REGEX", infraerrors.Reason(err))
			require.Equal(t, compilerErr.Error(), infraerrors.Message(err))
		})
	}
}
