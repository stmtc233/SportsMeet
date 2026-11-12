package arrange

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

// Fingerprint identifies a heat's arrangement: ordinal, custom name, own
// scheduled time, expected duration and slots. The arrange page sends back
// the one it loaded; a different one now means the heat was changed
// elsewhere. The round's start is not part of it: it changes no heat.
func Fingerprint(h domain.Heat) string {
	var b strings.Builder
	duration := ""
	if h.DurationMinutes != nil {
		duration = fmt.Sprint(*h.DurationMinutes)
	}
	fmt.Fprintf(&b, "%d\x00%s\x00%s\x00%s", h.Ordinal, h.Name, h.ScheduledAt, duration)
	for _, s := range h.Slots {
		fmt.Fprintf(&b, "\x00%d=%s", s.Position, s.AthleteID+"|"+s.TeamID)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:12])
}

// DeriveOrdinals numbers heats in their new order while changing as few as
// possible: a heat keeps its ordinal while the sequence still increases,
// otherwise it takes the previous one + 1. New heats pass 0.
func DeriveOrdinals(current []int) []int {
	out := make([]int, len(current))
	prev := 0
	for i, c := range current {
		if c <= prev {
			c = prev + 1
		}
		out[i] = c
		prev = c
	}
	return out
}
