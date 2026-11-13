package scoring

import (
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
)

func TestHeightTrialSequences(t *testing.T) {
	for _, text := range []string{"O", "XO", "XXO", "X", "XX", "XXX", "-", "X-", "XX-", "---", "x0", "×o", "－"} {
		if _, err := ParseTrials(text); err != nil {
			t.Errorf("%q: %v", text, err)
		}
	}
	for _, text := range []string{"OO", "OX", "O-", "XOX", "-O", "X-X", "XXXX", "1", "1.00"} {
		if _, err := ParseTrials(text); err == nil {
			t.Errorf("accepted %q", text)
		}
	}
	for _, tc := range []struct {
		trials []string
		valid  bool
	}{
		{[]string{"XX-", "X"}, true},
		{[]string{"XX-", "XO"}, false},
		{[]string{"XXX", "O"}, false},
		{[]string{"XX-", "-", "O"}, true},
		{[]string{"XO", "XXO", "XXX"}, true},
		{[]string{"XO", "XX-", "XO"}, false},
	} {
		bars := []domain.Bar{}
		for i, trials := range tc.trials {
			bars = append(bars, domain.Bar{Height: int64(1500 + i*50), Trials: trials})
		}
		if err := ValidateBars(bars); (err == nil) != tc.valid {
			t.Errorf("%v: %v", tc.trials, err)
		}
	}
}
