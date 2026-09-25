package level

// NightmareExpReward implements the three map branches of MobKilled.cpp:443-875.
// Each living party member in the killer's map gets its own GetExpApply result;
// it is not divided by party size and the item bonus belongs to the killer.
func NightmareExpReward(kind int, mobExp int64, memberLevel, mobLevel int32, class uint8, killerBonus int32, ev ExpEvents) int64 {
	exp := ExpApply(mobExp, memberLevel, mobLevel)
	if exp <= 0 || exp > soloExpGate {
		return 0
	}
	myLevel := int64(memberLevel)
	if class != classMortal && class != classArch {
		myLevel += int64(MaxLevel) + 1
	}
	if class != classMortal && class != classArch {
		exp = applyTierDivisors(exp, myLevel, class)
		// The Normal branch repeats this celestial block verbatim. Keeping
		// both divisions matters even though normal admission is mortal-only.
		if kind == 0 {
			exp = applyTierDivisors(exp, myLevel, class)
		}
	} else if kind != 0 {
		if class == classMortal {
			limits := [7]int64{200, 300, 356, 370, 380, 390, 399}
			divisors := [7]float32{1, 1.03, 1.50, 2.15, 2.78, 3.70, 5.20}
			if kind == 2 {
				divisors = [7]float32{1, 0.84, 1.05, 1.63, 1.95, 2.55, 3.70}
			}
			for i, limit := range limits {
				if myLevel <= limit {
					exp = int64(float32(exp) / divisors[i])
					break
				}
			}
		} else {
			limits := [8]int64{200, 300, 356, 360, 370, 380, 390, 400}
			divisors := [8]float32{0.84, 0.72, 1.40, 4.75, 6.60, 15, 21, 35}
			for i, limit := range limits {
				if myLevel <= limit {
					if i < 5 {
						exp = int64(float32(exp) / divisors[i])
					} else {
						exp /= int64(divisors[i])
					}
					break
				}
			}
		}
	}
	exp = 6 * exp / 10
	if killerBonus > 0 && killerBonus < 500 {
		exp += exp * int64(killerBonus) / 100
	}
	if ev.NewbieEvent && memberLevel < 100 && !isCelestialTier(class) {
		exp += exp / 4
	}
	if ev.DoubleMode {
		exp *= 2
	}
	if !ev.KefraLive {
		exp /= 2
	}
	if ev.NewbieEvent {
		exp += exp * 15 / 100
	} else {
		exp -= exp * 15 / 100
	}
	return exp
}
