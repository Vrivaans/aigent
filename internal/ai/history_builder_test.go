package ai

import (
	"strings"
	"testing"

	"aigent/internal/ai/cache"
	"aigent/internal/database"
)

func TestLayer2HashDeterministicForSameInputs(t *testing.T) {
	session := database.Session{SessionGoals: "Refinar tests unitarios del módulo auth"}
	files := []database.SessionFile{
		{Filename: "notes.md", Content: "# Notas\n- caso A\n"},
		{Filename: "spec.txt", Content: "Requisitos v1"},
	}
	workspace := "\n=== ARCHIVO DEL WORKSPACE: main.go ===\npackage main\n"

	h1 := Layer2SHA256(session, files, workspace)
	h2 := Layer2SHA256(session, files, workspace)
	if h1 == "" {
		t.Fatal("expected non-empty hash")
	}
	if h1 != h2 {
		t.Fatalf("expected identical hashes, got %q vs %q", h1, h2)
	}
}

func TestLayer2HashStableRegardlessOfFileOrder(t *testing.T) {
	session := database.Session{SessionGoals: "same goals"}
	filesA := []database.SessionFile{
		{Filename: "z-last.txt", Content: "z"},
		{Filename: "a-first.txt", Content: "a"},
	}
	filesB := []database.SessionFile{
		{Filename: "a-first.txt", Content: "a"},
		{Filename: "z-last.txt", Content: "z"},
	}

	if gotA, gotB := Layer2SHA256(session, filesA, ""), Layer2SHA256(session, filesB, ""); gotA != gotB {
		t.Fatalf("file order should not affect hash: %q vs %q", gotA, gotB)
	}
}

func TestLayer2HashChangesWhenGoalsChange(t *testing.T) {
	files := []database.SessionFile{{Filename: "ctx.txt", Content: "static"}}
	hBefore := Layer2SHA256(database.Session{SessionGoals: "Objetivo A"}, files, "")
	hAfter := Layer2SHA256(database.Session{SessionGoals: "Objetivo B"}, files, "")
	if hBefore == hAfter {
		t.Fatalf("expected different hashes when goals change, both were %q", hBefore)
	}
}

func TestLayer2HashChangesWhenSessionFileChanges(t *testing.T) {
	session := database.Session{SessionGoals: "fixed goal"}
	h1 := Layer2SHA256(session, []database.SessionFile{{Filename: "a.txt", Content: "v1"}}, "")
	h2 := Layer2SHA256(session, []database.SessionFile{{Filename: "a.txt", Content: "v2"}}, "")
	if h1 == h2 {
		t.Fatal("expected hash to change when file content changes")
	}
}

func TestLayer2HashChangesWhenWorkspaceChanges(t *testing.T) {
	session := database.Session{SessionGoals: "fixed goal"}
	ws1 := "\n=== ARCHIVO DEL WORKSPACE: a.go ===\npackage a\n"
	ws2 := "\n=== ARCHIVO DEL WORKSPACE: a.go ===\npackage b\n"
	if Layer2SHA256(session, nil, ws1) == Layer2SHA256(session, nil, ws2) {
		t.Fatal("expected hash to change when workspace content changes")
	}
}

func TestBuildRuntimeMessagesWithCacheIncludesLayer2(t *testing.T) {
	session := database.Session{SessionGoals: "test goals"}
	files := []database.SessionFile{{Filename: "doc.md", Content: "hello"}}
	layer2 := buildLayer2Content(session, files, "")
	plan := cache.Layer2Plan{
		IncludeInMessages: true,
		Strategy:          string(cache.FamilyPrefixStable),
	}
	msgs := buildRuntimeMessagesWithCache("sys", layer2, plan, nil, "hi")
	if len(msgs) < 2 {
		t.Fatalf("expected at least system + layer2, got %d messages", len(msgs))
	}
	if !strings.Contains(msgs[1].Content, "test goals") {
		t.Fatalf("layer 2 message missing goals: %q", msgs[1].Content)
	}
}
