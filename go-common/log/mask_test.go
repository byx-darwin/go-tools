package log_test

import (
	"log/slog"
	"testing"

	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/stretchr/testify/require"
)

func TestMasker_FullMask(t *testing.T) {
	cfg := log.MaskConfig{
		Enabled:      true,
		MaskedFields: []string{"password"},
		Mode:         "full",
	}
	masker := log.NewMasker(cfg)
	attrs := []slog.Attr{
		slog.String("username", "alice"),
		slog.String("password", "secret123"),
	}
	masked := masker.Mask(attrs)
	require.Equal(t, "alice", masked[0].Value.String())
	require.Equal(t, "***", masked[1].Value.String())
}

func TestMasker_Disabled(t *testing.T) {
	cfg := log.MaskConfig{
		Enabled:      false,
		MaskedFields: []string{"password"},
		Mode:         "full",
	}
	masker := log.NewMasker(cfg)
	attrs := []slog.Attr{
		slog.String("password", "secret123"),
	}
	masked := masker.Mask(attrs)
	require.Equal(t, "secret123", masked[0].Value.String())
}

func TestMasker_PartialMask(t *testing.T) {
	cfg := log.MaskConfig{
		Enabled:      true,
		MaskedFields: []string{"password"},
		Mode:         "partial",
	}
	masker := log.NewMasker(cfg)
	attrs := []slog.Attr{
		slog.String("password", "secret123"),
	}
	masked := masker.Mask(attrs)
	require.Equal(t, "se***23", masked[0].Value.String())
}

func TestMasker_PartialMask_ShortValue(t *testing.T) {
	cfg := log.MaskConfig{
		Enabled:      true,
		MaskedFields: []string{"password"},
		Mode:         "partial",
	}
	masker := log.NewMasker(cfg)
	attrs := []slog.Attr{
		slog.String("password", "ab"),
	}
	masked := masker.Mask(attrs)
	require.Equal(t, "***", masked[0].Value.String())
}

func TestMasker_MultipleFields(t *testing.T) {
	cfg := log.MaskConfig{
		Enabled:      true,
		MaskedFields: []string{"password", "token"},
		Mode:         "full",
	}
	masker := log.NewMasker(cfg)
	attrs := []slog.Attr{
		slog.String("username", "alice"),
		slog.String("password", "secret123"),
		slog.String("token", "abc123"),
	}
	masked := masker.Mask(attrs)
	require.Equal(t, "alice", masked[0].Value.String())
	require.Equal(t, "***", masked[1].Value.String())
	require.Equal(t, "***", masked[2].Value.String())
}

func TestMasker_CaseInsensitive(t *testing.T) {
	cfg := log.MaskConfig{
		Enabled:      true,
		MaskedFields: []string{"Password"},
		Mode:         "full",
	}
	masker := log.NewMasker(cfg)
	attrs := []slog.Attr{
		slog.String("password", "secret123"),
	}
	masked := masker.Mask(attrs)
	require.Equal(t, "***", masked[0].Value.String())
}
