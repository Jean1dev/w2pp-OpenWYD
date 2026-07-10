package content

import (
	"path/filepath"
	"testing"
)

func TestLoadBaseSummons(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "Release")
	templates, missing, err := LoadBaseSummons(dir)
	if err != nil {
		t.Skipf("BaseSummon tree unavailable: %v", err)
	}
	if len(templates) != 40 {
		t.Fatalf("templates = %d slots, want 40 (CNPCSummon::Initialize order)", len(templates))
	}
	// The 8 BM evocation creatures are mandatory and byte-sized like any
	// STRUCT_MOB template.
	for i := 0; i < bmSummonCount; i++ {
		if templates[i] == nil {
			t.Errorf("BM summon %d (%s) missing", i, baseSummonFiles[i])
			continue
		}
		if len(templates[i]) != BaseMobSize {
			t.Errorf("summon %d (%s) = %d bytes, want %d", i, baseSummonFiles[i], len(templates[i]), BaseMobSize)
		}
	}
	// The case-corrected mount template must load on Linux (the C++ says
	// "Dragao_menor"; the file is Dragao_Menor).
	if templates[13] == nil {
		t.Errorf("summon 13 (Dragao_Menor) missing — case fix regressed?")
	}
	for _, name := range missing {
		t.Logf("optional summon missing: %s", name)
	}
}
