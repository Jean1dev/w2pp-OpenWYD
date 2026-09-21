package handler

import "github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"

// mobDeathExpLoss is the legacy PvE death penalty from MobKilled.cpp. The
// server subtracts it immediately; the client is refreshed by sendEtc.
func mobDeathExpLoss(victimLevel int32, classMaster, pkPoint uint8) int64 {
	// New mortal characters are protected by the legacy FREEEXP gate.
	if classMaster == classMasterMortal && victimLevel < 35 {
		return 0
	}

	alpha := level.NextLevelExpTier(victimLevel, classMaster) -
		level.NextLevelExpTier(victimLevel-1, classMaster)
	if alpha <= 0 {
		return 0
	}
	divisor := int64(20)
	switch {
	case victimLevel >= 250:
		divisor = 100
	case victimLevel >= 200:
		divisor = 85
	case victimLevel >= 150:
		divisor = 70
	case victimLevel >= 100:
		divisor = 55
	case victimLevel >= 90:
		divisor = 50
	case victimLevel >= 80:
		divisor = 45
	case victimLevel >= 70:
		divisor = 40
	case victimLevel >= 60:
		divisor = 35
	case victimLevel >= 50:
		divisor = 30
	case victimLevel >= 40:
		divisor = 25
	case victimLevel >= 30:
		divisor = 22
	}
	loss := alpha / divisor
	if loss > 150000 {
		loss = 150000
	}
	if pkPoint > 10 && pkPoint <= 25 {
		loss *= 3
	} else {
		loss *= 5
	}
	if loss > 30000 {
		loss = 30000
	}
	return loss
}
