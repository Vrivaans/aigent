package utils

import (
	"testing"
)

func TestExtractArtifacts(t *testing.T) {
	input := `Hola usuario, aquí está el diagrama que pediste.

<artifact type="diagram" format="mermaid" title="Flujo de Login" id="diag-login-123">
graph TD
    A[Inicio] --> B(Login)
    B --> C{Correcto?}
    C -- Sí --> D[Home]
    C -- No --> B
</artifact>

Espero que te sea de utilidad.`

	clean, arts := ExtractArtifacts(input)

	expectedClean := `Hola usuario, aquí está el diagrama que pediste.



Espero que te sea de utilidad.`

	if clean != expectedClean {
		t.Errorf("Expected clean text:\n%q\nGot:\n%q", expectedClean, clean)
	}

	if len(arts) != 1 {
		t.Fatalf("Expected 1 artifact, got %d", len(arts))
	}

	art := arts[0]
	if art.Type != "diagram" {
		t.Errorf("Expected Type='diagram', got %q", art.Type)
	}
	if art.Format != "mermaid" {
		t.Errorf("Expected Format='mermaid', got %q", art.Format)
	}
	if art.Title != "Flujo de Login" {
		t.Errorf("Expected Title='Flujo de Login', got %q", art.Title)
	}
	if art.ID != "diag-login-123" {
		t.Errorf("Expected ID='diag-login-123', got %q", art.ID)
	}
	expectedContent := `graph TD
    A[Inicio] --> B(Login)
    B --> C{Correcto?}
    C -- Sí --> D[Home]
    C -- No --> B`

	if art.Content != expectedContent {
		t.Errorf("Expected Content:\n%q\nGot:\n%q", expectedContent, art.Content)
	}
}
