package combat

const hitRateSize = 1024

var hitRate = initHitRate()

func initHitRate() [hitRateSize]int {
	var table [hitRateSize]int
	jump := 512
	start := 0
	quad := 0
	for {
		for i := 0; i < hitRateSize; i++ {
			if table[i] != 0 {
				continue
			}
			switch quad {
			case 0:
				table[i] = start
			case 1:
				table[i] = 512 - start
			case 2:
				table[i] = start + 512
			default:
				table[i] = 1024 - start
			}
			if table[i] > 999 {
				table[i] = 999
			}
			quad++
			if quad >= 4 {
				quad = 0
			}
			if quad == 0 {
				start++
			}
		}
		jump /= 2
		if jump == 0 {
			break
		}
	}
	table[0] = 512
	return table
}

// DoubleCritical ports BASE_GetDoubleCritical. It mutates both progress counters
// exactly like the legacy routine and returns the authoritative bitfield:
// bit 0 = total critical, bit 1 = partial critical.
func DoubleCritical(r Rand, attackRun uint8, critical int, serverProgress, clientProgress *uint16) (uint8, bool) {
	if clientProgress == nil {
		return 0, false
	}
	ret := true
	if serverProgress != nil && *clientProgress != *serverProgress {
		*serverProgress = *clientProgress
	}
	value := hitRate[int(*clientProgress)&(hitRateSize-1)]
	var flags uint8
	if value < 100*(int(attackRun>>4)-5) {
		flags |= 1
	}
	if r.Intn(255) < critical {
		flags |= 2
	}
	if serverProgress != nil {
		*serverProgress = *serverProgress + 1
	}
	*clientProgress = *clientProgress + 1
	return flags, ret
}

// ParryRate ports GetParryRate. The return value is in thousandths and is
// clamped to the legacy [1,650] range.
func ParryRate(targetDex, add, attackerDex, attackerRsv int) int {
	if add > 100 {
		add = 100
	}
	if add < 0 {
		add = 0
	}
	tdex := targetDex
	if tdex > 1000 {
		tdex = 1000
	}
	parry1 := targetDex - 1000
	if parry1 < 0 {
		parry1 = 0
	}
	if parry1 >= 2000 {
		parry1 = 2000
	}
	parry2 := targetDex - 3000
	if parry2 < 0 {
		parry2 = 0
	}
	parry := tdex/2 + add + parry1/4 + parry2/8 - attackerDex
	if attackerRsv&0x20 != 0 {
		parry += 100
	}
	if attackerRsv&0x80 != 0 {
		parry += 50
	}
	if attackerRsv&0x200 != 0 {
		parry += 50
	}
	if parry >= 650 {
		parry = 650
	}
	if parry < 1 {
		parry = 1
	}
	return parry
}

// ResolveParry consumes the legacy rand()%1000+1 roll and returns 0 for a hit,
// -3 for a parry/miss, or -4 for a blocked low parry roll.
func ResolveParry(r Rand, skillIndex, parryRate int, targetRsvBlock bool) int {
	parry := parryRate
	if skillIndex == 79 || skillIndex == 22 {
		parry = 30 * parry / 100
	}
	rd := r.Intn(1000) + 1
	if rd < parry {
		if targetRsvBlock && rd < 100 {
			return -4
		}
		return -3
	}
	return 0
}
