package agents

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/eru-tech/eru/eru-ai/models"
)

func TestNoBudgetMeansNoLimit(t *testing.T) {
	var none Budget
	spend := Spend{Attempts: 10000, Tokens: 1 << 40, Started: time.Now().Add(-48 * time.Hour)}
	if reason, why := none.Exceeded(context.Background(), spend); reason != "" {
		t.Errorf("an agent that declares nothing must be unbounded as before: %s %s", reason, why)
	}
}

func TestEachLimitStopsForItsOwnReason(t *testing.T) {
	cases := []struct {
		name   string
		budget Budget
		spend  Spend
		want   StopReason
	}{
		{"attempts", Budget{MaxAttempts: 3}, Spend{Attempts: 3}, StopLimitAttempts},
		{"tokens", Budget{MaxTotalTokens: 1000}, Spend{Tokens: 1000}, StopLimitTokens},
		{"duration", Budget{MaxDurationSeconds: 1}, Spend{Started: time.Now().Add(-2 * time.Second)}, StopLimitDuration},
	}
	for _, c := range cases {
		reason, why := c.budget.Exceeded(context.Background(), c.spend)
		if reason != c.want {
			t.Errorf("%s: reason = %q, want %q", c.name, reason, c.want)
		}
		if why == "" {
			t.Errorf("%s: a limit must say what it was, or nobody can raise it", c.name)
		}
	}
}

func TestJustUnderALimitIsAllowed(t *testing.T) {
	budget := Budget{MaxAttempts: 3, MaxTotalTokens: 1000, MaxDurationSeconds: 60}
	spend := Spend{Attempts: 2, Tokens: 999, Started: time.Now()}
	if reason, _ := budget.Exceeded(context.Background(), spend); reason != "" {
		t.Errorf("the last permitted attempt must be permitted, got %q", reason)
	}
}

// A cancelled run is not a broken one; counting it as a failure makes a deploy
// look like a regression.
func TestCancellationIsItsOwnReason(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reason, why := Budget{}.Exceeded(ctx, Spend{Attempts: 1})
	if reason != StopCancelled {
		t.Fatalf("reason = %q, want cancelled", reason)
	}
	if !strings.Contains(why, "went away") {
		t.Errorf("why = %q", why)
	}
}

// Cancellation is checked before the declared limits: "nobody is waiting" is a
// truer description than "it ran out of tokens", even when both hold.
func TestCancellationOutranksALimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if reason, _ := (Budget{MaxAttempts: 1}).Exceeded(ctx, Spend{Attempts: 5}); reason != StopCancelled {
		t.Errorf("reason = %q, want cancelled", reason)
	}
}

func TestSpendPrefersTheReportedTotal(t *testing.T) {
	var spend Spend
	spend.Add(&models.TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 20})
	if spend.Tokens != 20 {
		t.Errorf("a reported total wins over the parts: %d", spend.Tokens)
	}
	// Not every provider reports one.
	spend.Add(&models.TokenUsage{InputTokens: 10, OutputTokens: 5, ReasoningTokens: 2})
	if spend.Tokens != 37 {
		t.Errorf("the parts must be summed when no total is given: %d", spend.Tokens)
	}
	spend.Add(nil)
	if spend.Tokens != 37 {
		t.Error("a missing usage must not corrupt the count")
	}
}

// Only three reasons mean "nothing was produced". Getting this wrong would make
// a delivered answer look like a failure, or the reverse.
func TestOnlyTheEmptyHandedReasonsAreTerminal(t *testing.T) {
	terminal := map[StopReason]bool{
		StopValidationExhausted: true, StopModelError: true, StopCancelled: true,
	}
	for _, reason := range []StopReason{
		StopEndTurn, StopAskedUser, StopValidationExhausted, StopRecoveredEarlierAnswer,
		StopLimitAttempts, StopLimitTokens, StopLimitDuration, StopModelError, StopCancelled,
	} {
		if reason.Terminal() != terminal[reason] {
			t.Errorf("%s.Terminal() = %v", reason, reason.Terminal())
		}
	}
}

// The first reason decided is the true one; a later default must not paper it over.
func TestTheFirstReasonWins(t *testing.T) {
	message := &AgentMessage{}
	StopLimitTokens.Set(message)
	StopEndTurn.Set(message)
	if message.StopReason != StopLimitTokens {
		t.Errorf("StopReason = %q, want limit_total_tokens", message.StopReason)
	}
}
