package level

// BasePools is BASE_GetHpMp (Basedef.cpp:1214-1246): the equipment-free MaxHP
// and MaxMP of a character, rebuilt from its class, tier, level and the CON/INT
// it bought with points.
//
//	HP = BaseSIDCHM HP + (Con − class Con)×2 + levelTerm × IncHP
//	MP = BaseSIDCHM MP + (Int − class Int)×2 + levelTerm × IncMP
//
// levelTerm is the stored level for a Mortal or an Arch, and level + MAX_LEVEL
// (399) for the celestial tiers. That offset is the whole point of the function
// for a Celestial: it is born at level 0 with the pools of 399 levels already
// in it — the same level share an Arch 400 carries, without the attribute
// points. A port that kept only the Mortal branch left every Celestial with a
// level-1 character's pools (an FM with 65 MP where the original gives 1,262).
//
// con and intel are the BASE attributes (without equipment), as the original
// reads BaseScore. Both pools stop at MAX_HP / MAX_MP.
func BasePools(cls, classMaster uint8, lvl, con, intel int32) (hp, mp int32) {
	c := validClass(cls)
	base := baseSIDCHM[c]
	levelTerm := lvl
	if isCelestialTier(classMaster) {
		levelTerm += MaxLevel
	}
	hp = min(base[4]+(con-base[3])*2+levelTerm*incHP[c], MaxHPCap)
	mp = min(base[5]+(intel-base[1])*2+levelTerm*incMP[c], MaxMPCap)
	return hp, mp
}
