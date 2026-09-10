package clientrarity

import (
	"encoding/binary"
	"testing"
)

func TestClassify(t *testing.T) {
	for _, tc := range []struct {
		name string
		it   Item
		want Tier
	}{
		{"armadura (N)", Item{Pos: posHelm, Grade: 1, ReqLevel: 48}, Comum},
		{"armadura (M)", Item{Pos: posBody, Grade: 2, ReqLevel: 64}, Raro},
		{"armadura (A)", Item{Pos: posLegs, Grade: 3, ReqLevel: 64}, Epico},
		{"armadura (Le)", Item{Pos: posBoots, Grade: 4, ReqLevel: 158}, Lendario},
		{"Arch vence a letra (N)", Item{Pos: posHelm, Grade: 1, MobType: mobTypeArch}, Lendario},
		{"arma Arch", Item{Pos: posBoth, MobType: mobTypeArch, ReqLevel: 342}, Lendario},
		{"Celestial", Item{Pos: posGloves, Grade: 1, MobType: mobTypeCelestial}, Mitico},
		{"Ancient Mortal", Item{Pos: posRight, Grade: 5, ReqLevel: 40}, Divino},
		{"Ancient vence Celestial", Item{Pos: posRight, Grade: 8, MobType: mobTypeCelestial}, Divino},
		{"arma lv 99", Item{Pos: posRight, ReqLevel: 99}, Comum},
		{"arma lv 100", Item{Pos: posRight, ReqLevel: 100}, Incomum},
		{"arma lv 150", Item{Pos: posLeft, ReqLevel: 150}, Raro},
		{"arma lv 200", Item{Pos: posBoth, ReqLevel: 200}, Epico},
		// Lendário is kept for (Le) and Arch: a Mortal weapon tops out at Épico.
		{"arma Mortal lv 350", Item{Pos: posBoth, ReqLevel: 350}, Epico},
		{"Demolidor Celestial, exceção", Item{Index: 3596, Pos: posRight, ReqLevel: 312, MobType: 2}, Incomum},
		{"Demolidor Celestial (Anct) segue Divino", Item{Index: 3781, Pos: posRight, ReqLevel: 341, Grade: 5, MobType: 2}, Divino},
		{"poção", Item{Pos: 0, ReqLevel: 0}, None},
		{"anel", Item{Pos: 256, Grade: 3}, None},
		{"montaria", Item{Pos: 16384, ReqLevel: 240}, None},
	} {
		if got := Classify(tc.it); got != tc.want {
			t.Errorf("%s: Classify = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestWithRefine(t *testing.T) {
	for _, tc := range []struct {
		name   string
		base   Tier
		refine int
		want   Tier
	}{
		{"Comum +10 fica Comum", Comum, 10, Comum},
		{"Comum +11 vira Épico", Comum, 11, Epico},
		{"Raro +12 vira Épico", Raro, 12, Epico},
		{"Comum +13 vira Lendário", Comum, 13, Lendario},
		{"Épico +14 vira Mítico", Epico, 14, Mitico},
		{"Comum +15 vira Divino", Comum, 15, Divino},
		{"Mítico +11 não desce", Mitico, 11, Mitico},
		{"Divino +13 não desce", Divino, 13, Divino},
		{"fora da tabela continua fora", None, 15, None},
		{"+9 não muda", Raro, 9, Raro},
	} {
		if got := WithRefine(tc.base, tc.refine); got != tc.want {
			t.Errorf("%s: WithRefine = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// fakeItemList builds a client catalog with the given records filled in.
func fakeItemList(recs map[int]func(rec []byte)) []byte {
	b := make([]byte, itemListSize)
	for i := range b {
		b[i] = itemXOR // registro vazio: zero depois do XOR
	}
	for i, fill := range recs {
		rec := make([]byte, itemRecord)
		fill(rec)
		for j := range rec {
			b[i*itemRecord+j] = rec[j] ^ itemXOR
		}
	}
	return b
}

func TestReadItemListETable(t *testing.T) {
	il := fakeItemList(map[int]func([]byte){
		1103: func(rec []byte) { // Elmo_de_Couro(A)
			copy(rec, "Elmo_de_Couro(A)")
			binary.LittleEndian.PutUint16(rec[offPos:], posHelm)
			binary.LittleEndian.PutUint16(rec[offGrade:], 3)
		},
		811: func(rec []byte) { // Balmung, arma Arch
			copy(rec, "Balmung")
			binary.LittleEndian.PutUint16(rec[offReqLevel:], 342)
			binary.LittleEndian.PutUint16(rec[offPos:], posBoth)
			// EF_CLASS no slot 1 e EF_MOBTYPE no 2: o tipo é achado em qualquer slot.
			binary.LittleEndian.PutUint16(rec[offEffects+4:], 18)
			binary.LittleEndian.PutUint16(rec[offEffects+8:], efMobType)
			binary.LittleEndian.PutUint16(rec[offEffects+10:], mobTypeArch)
		},
	})
	items, err := ReadItemList(il)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("itens lidos = %d, want 2 (registro vazio é pulado)", len(items))
	}
	byIndex := map[int]Item{}
	for _, it := range items {
		byIndex[it.Index] = it
	}
	if it := byIndex[811]; it.Name != "Balmung" || it.ReqLevel != 342 || it.MobType != mobTypeArch || it.Pos != posBoth {
		t.Errorf("Balmung lido como %+v", it)
	}

	tab := Table(items)
	if string(tab[:4]) != fileMagic || binary.LittleEndian.Uint16(tab[4:]) != itemCount || len(tab) != fileHeaderSize+itemCount {
		t.Fatalf("cabeçalho da tabela errado: % X, tamanho %d", tab[:fileHeaderSize], len(tab))
	}
	// +10 … +15: nada, Épico, Épico, Lendário, Mítico, Divino.
	wantFloors := []byte{6, byte(None), byte(Epico), byte(Epico), byte(Lendario), byte(Mitico), byte(Divino)}
	if got := tab[6:fileHeaderSize]; string(got) != string(wantFloors) {
		t.Errorf("pisos de refinação = % X, want % X", got, wantFloors)
	}
	for idx, want := range map[int]Tier{1103: Epico, 811: Lendario, 0: None, 2360: None} {
		if got := Tier(tab[fileHeaderSize+idx]); got != want {
			t.Errorf("tabela[%d] = %v, want %v", idx, got, want)
		}
	}
}

func TestReadItemListRecusaTamanhoErrado(t *testing.T) {
	if _, err := ReadItemList(make([]byte, itemListSize-1)); err == nil {
		t.Error("aceitou um ItemList.bin de tamanho errado")
	}
}
