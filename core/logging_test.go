package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeLogValue(t *testing.T) {
	t.Parallel()

	require.Equal(t, "first second third", SanitizeLogValue("first\nsecond\rthird"))
	require.Equal(t, "bad?byte", SanitizeLogValue("bad\x00byte"))
}

func TestSanitizeLogValueShouldBoundOutput(t *testing.T) {
	t.Parallel()

	sanitized := SanitizeLogValue(strings.Repeat("a", maxSanitizedLogValueLength+10))

	require.Len(t, sanitized, maxSanitizedLogValueLength+3)
	require.True(t, strings.HasSuffix(sanitized, "..."))
}

func TestSanitizeLogError(t *testing.T) {
	t.Parallel()

	require.Empty(t, SanitizeLogError(nil))
	require.Equal(t, "boom injected", SanitizeLogError(errors.New("boom\ninjected")))
}
