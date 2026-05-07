package tasks

import (
	"testing"
	"time"
)

func TestCalculateNextRunEveryMinute(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	next := CalculateNextRun("* * * * *", now)
	if next.Before(now) {
		t.Fatal("Next run should be after now")
	}
	if next.Sub(now) > time.Minute || next.Sub(now) <= 0 {
		t.Fatalf("Expected next run ~1 minute later, got %v", next.Sub(now))
	}
}

func TestCalculateNextRunEveryHour(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	next := CalculateNextRun("@hourly", now)
	if next.Before(now) {
		t.Fatal("Next run should be after now")
	}
	if next.Sub(now) > time.Hour || next.Sub(now) <= 0 {
		t.Fatalf("Expected next run ~1 hour later, got %v", next.Sub(now))
	}
}

func TestCalculateNextRunDaily(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	next := CalculateNextRun("@daily", now)
	if next.Before(now) {
		t.Fatal("Next run should be after now")
	}
	if next.Sub(now) > 24*time.Hour || next.Sub(now) <= 0 {
		t.Fatalf("Expected next run within 24h, got %v", next.Sub(now))
	}
}

func TestCalculateNextRunSpecificTime(t *testing.T) {
	// "0 9 * * *" = every day at 9:00 AM
	now := time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC)
	next := CalculateNextRun("0 9 * * *", now)

	expected := time.Date(2025, 6, 1, 9, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("Expected next run at 9:00 AM, got %v", next)
	}
}

func TestCalculateNextRunSpecificTimeAlreadyPassed(t *testing.T) {
	// "0 9 * * *" = every day at 9:00 AM, now is 10:00 AM → tomorrow 9:00
	now := time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)
	next := CalculateNextRun("0 9 * * *", now)

	expected := time.Date(2025, 6, 2, 9, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("Expected next run at tomorrow 9:00 AM, got %v", next)
	}
}

func TestCalculateNextRunInvalidExpr(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	next := CalculateNextRun("not-a-valid-cron", now)
	// Falls back to 24h
	if next.Sub(now) != 24*time.Hour {
		t.Fatalf("Expected fallback of 24h, got %v", next.Sub(now))
	}
}

func TestCalculateNextRunDescriptor(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	next := CalculateNextRun("@every 5m", now)
	if next.Before(now) {
		t.Fatal("Next run should be after now")
	}
	diff := next.Sub(now)
	if diff > 5*time.Minute || diff <= 0 {
		t.Fatalf("Expected ~5 min later, got %v", diff)
	}
}

func TestCalculateNextRunMidnightDaily(t *testing.T) {
	now := time.Date(2025, 6, 1, 23, 0, 0, 0, time.UTC)
	next := CalculateNextRun("@daily", now)

	expected := time.Date(2025, 6, 2, 0, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("Expected next run at midnight tomorrow, got %v", next)
	}
}
