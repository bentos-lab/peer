package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveBoolPrimary(t *testing.T) {
	primary := true
	fallback := false

	result := ResolveBool(&primary, &fallback, false)

	require.True(t, result)
}

func TestResolveBoolFallback(t *testing.T) {
	fallback := true

	result := ResolveBool(nil, &fallback, false)

	require.True(t, result)
}

func TestResolveBoolDefault(t *testing.T) {
	result := ResolveBool(nil, nil, true)

	require.True(t, result)
}
