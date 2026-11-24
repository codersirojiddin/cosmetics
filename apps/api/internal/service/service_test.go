package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizePhone(t *testing.T) {
	ok := map[string]string{
		"+998901234567":     "+998901234567",
		"998 90 123 45 67":  "+998901234567",
		"90 123 45 67":      "+998901234567",
		"(90) 123-45-67":    "+998901234567",
		"0901234567":        "+998901234567",
		"+998-90-123-45-67": "+998901234567",
	}
	for in, want := range ok {
		got, valid := normalizePhone(in)
		require.True(t, valid, in)
		require.Equal(t, want, got, in)
	}
	for _, in := range []string{"", "123", "+7 999 123 45 67", "90123456", "abc", "+99890123456789"} {
		_, valid := normalizePhone(in)
		require.False(t, valid, in)
	}
}

func TestSlugify(t *testing.T) {
	require.Equal(t, "yuz-parvarishi", slugify("Yuz parvarishi"))
	require.Equal(t, "hyaluron-serum-30-ml", slugify("  Hyaluron SERUM, 30 ml!! "))
	require.Equal(t, "oltin-qosh", slugify("Oltin qosh"))
	require.Equal(t, "krem-dlya-litsa", slugify("Крем для лица"))
	require.Equal(t, "gozal", slugify("Go‘zal")) // Uzbek apostrophes are dropped, not turned into dashes
	require.Equal(t, "gozal-ong", slugify("Goʻzal-oʼng"))
	require.Equal(t, "item", slugify("!!!"))
	require.LessOrEqual(t, len(slugify(strings.Repeat("a", 300))), 80)
}

func TestCleanMarkdown(t *testing.T) {
	in := "Salom <b>dunyo</b><script>alert(1)</script> [x](javascript:alert(1)) 3 < 5 va **qalin**"
	out := cleanMarkdown(in)
	require.NotContains(t, out, "<b>")
	require.NotContains(t, out, "<script")
	require.NotContains(t, strings.ToLower(out), "javascript:")
	require.Contains(t, out, "3 < 5")
	require.Contains(t, out, "**qalin**")
}

func TestFormatMoney(t *testing.T) {
	require.Equal(t, "0", formatMoney(0))
	require.Equal(t, "999", formatMoney(999))
	require.Equal(t, "85 000", formatMoney(85000))
	require.Equal(t, "1 250 000", formatMoney(1250000))
}

func TestStateMachine(t *testing.T) {
	allowed := map[[2]string]bool{
		{"waiting", "confirmed"}: true, {"waiting", "shipping"}: true, {"waiting", "rejected"}: true,
		{"confirmed", "shipping"}: true, {"confirmed", "rejected"}: true, {"shipping", "delivered"}: true,
	}
	all := []string{"waiting", "confirmed", "shipping", "delivered", "rejected"}
	for _, from := range all {
		for _, to := range all {
			require.Equal(t, allowed[[2]string{from, to}], CanTransition(from, to), "%s -> %s", from, to)
		}
	}
	require.True(t, IsTerminal("delivered"))
	require.True(t, IsTerminal("rejected"))
	require.False(t, IsTerminal("shipping"))
}
