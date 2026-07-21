package npctemplate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveExactName(t *testing.T) {
	dir := makeNPCDir(t)
	writeTemplate(t, dir, "Chefe_Treina.", Size)
	writeTemplate(t, dir, "Chefe_Treina", Size)

	res, err := Resolve(dir, "Chefe_Treina.")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Name != "Chefe_Treina." {
		t.Fatalf("resolved %q, want exact file", res.Name)
	}
}

func TestResolveTrimsLegacyTrailingDot(t *testing.T) {
	dir := makeNPCDir(t)
	writeTemplate(t, dir, "Chefe_Treina", Size)

	res, err := Resolve(dir, "Chefe_Treina.")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Name != "Chefe_Treina" {
		t.Fatalf("resolved %q, want Chefe_Treina", res.Name)
	}
}

func TestResolveCaseInsensitive(t *testing.T) {
	dir := makeNPCDir(t)
	writeTemplate(t, dir, "reiners", Size)

	res, err := Resolve(dir, "Reiners")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Name != "reiners" {
		t.Fatalf("resolved %q, want reiners", res.Name)
	}
}

func TestResolveAmbiguousLegacyName(t *testing.T) {
	dir := makeNPCDir(t)
	writeTemplate(t, dir, "Reiners", Size)
	writeTemplate(t, dir, "reiners", Size)

	if _, err := Resolve(dir, "REINERS"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("Resolve ambiguous err = %v, want ambiguity", err)
	}
}

func TestResolveRejectsPathTraversal(t *testing.T) {
	dir := makeNPCDir(t)
	for _, name := range []string{"../Chefe_Treina", `..\Chefe_Treina`, "npc/Chefe_Treina"} {
		if _, err := Resolve(dir, name); err == nil || !strings.Contains(err.Error(), "invalid template name") {
			t.Fatalf("Resolve(%q) err = %v, want invalid template name", name, err)
		}
	}
}

func TestLoadRejectsWrongSize(t *testing.T) {
	dir := makeNPCDir(t)
	writeTemplate(t, dir, "Broken", Size-1)

	if _, _, err := Load(dir, "Broken"); err == nil || !strings.Contains(err.Error(), "want 816") {
		t.Fatalf("Load err = %v, want size error", err)
	}
}

func makeNPCDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "TMsrv", "run", "npc"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeTemplate(t *testing.T, contentDir, name string, size int) {
	t.Helper()
	path := filepath.Join(contentDir, "TMsrv", "run", "npc", name)
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}
