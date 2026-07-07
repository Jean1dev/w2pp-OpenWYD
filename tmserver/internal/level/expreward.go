package level

const (
	classArch        uint8 = 1
	classMortal      uint8 = 2
	classCelestial   uint8 = 3
	classCelestialCS uint8 = 4
	classSCelestial  uint8 = 5

	soloExpCap int64 = 10_000_000
)

type ExpEvents struct {
	DoubleMode  bool
	NewbieEvent bool
	KefraLive   bool
}

func SoloExpReward(base int64, lvl int32, classMaster uint8, expBonus int32, ev ExpEvents) int64 {
	if base <= 0 {
		return 0
	}
	exp := float64(base)
	myLevel := lvl
	if classMaster != classMortal && classMaster != classArch {
		myLevel += MaxLevel + 1
	}
	exp = applyTierDivisors(exp, myLevel, classMaster)
	exp = exp * 6 / 10
	if expBonus > 0 && expBonus < 500 {
		exp += exp * float64(expBonus) / 100
	}
	if ev.NewbieEvent && lvl < 100 && !isCelestialTier(classMaster) {
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
	out := int64(exp)
	if out <= 0 {
		return 0
	}
	if out > soloExpCap {
		return soloExpCap
	}
	return out
}

func isCelestialTier(classMaster uint8) bool {
	switch classMaster {
	case classCelestial, classCelestialCS, classSCelestial:
		return true
	default:
		return false
	}
}

func applyTierDivisors(exp float64, myLevel int32, classMaster uint8) float64 {
	switch classMaster {
	case classMortal:
		switch {
		case myLevel <= 200:
		case myLevel <= 300:
			exp /= 0.84
		case myLevel <= 356:
			exp /= 1.05
		case myLevel <= 370:
			exp /= 1.63
		case myLevel <= 380:
			exp /= 1.95
		case myLevel <= 390:
			exp /= 2.55
		case myLevel <= 399:
			exp /= 3.70
		}
	case classArch:
		switch {
		case myLevel <= 200:
			exp /= 0.84
		case myLevel <= 300:
			exp /= 0.72
		case myLevel <= 356:
			exp /= 1.40
		case myLevel <= 360:
			exp /= 4.75
		case myLevel <= 370:
			exp /= 6.60
		case myLevel <= 380:
			exp /= 15
		case myLevel <= 390:
			exp /= 21
		case myLevel <= 400:
			exp /= 35
		}
	default:
		switch {
		case myLevel < 120:
			exp /= 10
		case myLevel < 150:
			exp /= 20
		case myLevel < 170:
			exp /= 40
		case myLevel < 180:
			exp /= 80
		case myLevel < 190:
			exp /= 160
		default:
			exp /= 320
		}
	}
	return exp
}
