package protocol

// Item effect ids used by the legacy visual-overlay helpers. These are wire
// item-effect bytes from ItemEffect.h, not score model constants.
const (
	visualEfSanc      = 43
	visualAncientLo   = 116
	visualAncientHi   = 125
	visualMountSlot   = 14
	visualMountLo     = 2360
	visualMountHi     = 2390
	visualSpecialLo   = 3980
	visualSpecialHi   = 3995
	visualAncientGlow = 12
)

// VisualEquip returns the two legacy visual codes for one equipped STRUCT_ITEM:
// MSG_CreateMob/MSG_UpdateEquip.Equip[] and AnctCode[].
func VisualEquip(it SelItem, slot int) (uint16, uint8) {
	code := VisualItemCode(it, slot)
	anct := VisualAnctCode(it)

	if slot == visualMountSlot && code >= visualMountLo && code < visualMountHi {
		if effectSValue(it, 0) <= 0 {
			return 0, anct
		}
		mountLevel := int(it.Eff[1][0]) / 10
		if mountLevel > 13 {
			mountLevel = 13
		}
		if mountLevel < 0 {
			mountLevel = 0
		}
		code += uint16(mountLevel * 0x1000)
	}

	return code, anct
}

// VisualItemCode mirrors BASE_VisualItemCode: the item id plus the high-nibble
// refine/ancient overlay that selects the rendered model variant.
func VisualItemCode(it SelItem, slot int) uint16 {
	if it.Index == 0 {
		return 0
	}
	if slot == visualMountSlot {
		if it.Index >= visualSpecialLo && it.Index < visualSpecialHi {
			return it.Index
		}
		return it.Index
	}

	value, ok := visualSancValue(it)
	if !ok {
		if hasAncientEffect(it) {
			return it.Index | uint16(visualAncientGlow*0x1000)
		}
		return it.Index
	}

	if value < 230 {
		value %= 10
	} else if value < 234 {
		value = 10
	} else if value < 238 {
		value = 11
	} else if value < 242 {
		value = 12
	} else if value < 246 {
		value = 13
	} else if value < 250 {
		value = 14
	} else if value < 254 {
		value = 15
	} else {
		value = 16
	}

	return it.Index | uint16(value*0x1000)
}

// VisualAnctCode mirrors BASE_VisualAnctCode: the one-byte glow/effect overlay
// the 7662 client reads beside the visual item id.
func VisualAnctCode(it SelItem) uint8 {
	if it.Index == 0 {
		return 0
	}
	if it.Index >= visualMountLo && it.Index <= visualMountHi && effectSValue(it, 0) > 0 {
		switch it.Eff[2][1] {
		case 11:
			return legacyByte(0x10B)
		case 12:
			return legacyByte(0x10C)
		case 13:
			return legacyByte(0x10D)
		case 14:
			return legacyByte(0x10E)
		case 15:
			return legacyByte(0x10F)
		case 16:
			return legacyByte(0x110)
		case 17:
			return legacyByte(0x111)
		case 18:
			return legacyByte(0x112)
		case 19:
			return legacyByte(0x113)
		case 20:
			return legacyByte(0x114)
		case 35:
			if it.Index == 2363 || it.Index == 2377 {
				return legacyByte(0x115)
			}
		default:
			return 0
		}
	}

	value, _ := visualSancValue(it)
	for _, ef := range it.Eff {
		if ef[0] >= visualAncientLo && ef[0] <= visualAncientHi {
			return ef[0] - 3
		}
	}
	if value == 0 {
		return 0
	}
	if value < 230 {
		return visualEfSanc
	}

	switch value % 4 {
	case 0:
		return 0x30
	case 1:
		return 0x40
	case 2:
		return 0x10
	default:
		return 0x20
	}
}

func visualSancValue(it SelItem) (int, bool) {
	for _, ef := range it.Eff {
		if ef[0] == visualEfSanc {
			return int(ef[1]), true
		}
	}
	return 0, false
}

func hasAncientEffect(it SelItem) bool {
	for _, ef := range it.Eff {
		if ef[0] >= visualAncientLo && ef[0] <= visualAncientHi {
			return true
		}
	}
	return false
}

func effectSValue(it SelItem, slot int) int16 {
	return int16(uint16(it.Eff[slot][0]) | uint16(it.Eff[slot][1])<<8)
}

func legacyByte(v int) uint8 {
	return uint8(v)
}
