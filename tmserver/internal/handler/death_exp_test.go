package handler

import "testing"

func TestMobDeathExpLoss(t *testing.T) {
	tests := []struct {
		name                 string
		level                int32
		classMaster, pkPoint uint8
		want                 int64
	}{
		{"protected mortal", 34, classMasterMortal, 75, 0},
		{"level 35 clean", 35, classMasterMortal, 75, 3970},
		{"mild chaos", 35, classMasterMortal, 20, 2382},
		{"high level cap", 399, classMasterMortal, 75, 30000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mobDeathExpLoss(tt.level, tt.classMaster, tt.pkPoint); got != tt.want {
				t.Fatalf("mobDeathExpLoss() = %d, want %d", got, tt.want)
			}
		})
	}
}
