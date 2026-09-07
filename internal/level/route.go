package level

// RouteStop is one place on a levelling route: a zone and the mob being farmed
// there. The tier, the bonuses and the configuration come from PlanRoute, so
// every stop is priced for the same character.
type RouteStop struct {
	Label  string
	Zone   Zone
	MobExp int64
	// MobLevel matters as much as MobExp: GetExpApply scales by the level ratio,
	// which is what makes a low mob stop paying a high character.
	MobLevel int32

	// FromLevel is the character level this stop opens at, and it is the
	// difference between a toy and a plan. Without it the walk hands level 1 the
	// richest monster on the route — a level-380 Pesadelo mob pays a beginner
	// enormously, because GetExpApply caps the ratio at 200% instead of refusing
	// — and reports a climb nobody could survive. Zero means "from the start".
	FromLevel int32
}

// RouteStep is one character level of the walk.
type RouteStep struct {
	Level int32
	// Stop is the index of the stop that paid best at this level, which is the
	// one a player would actually be standing in.
	Stop       int
	ExpPerKill int64
	ExpSpan    int64
	Kills      int64
}

// RouteBand folds a run of levels that shared one stop into a single row: the
// stretch of the climb somebody spends in one place.
type RouteBand struct {
	Stop     int
	From, To int32
	Kills    int64
}

// RoutePlan is the answer to "how long does this route take".
type RoutePlan struct {
	Steps      []RouteStep
	Bands      []RouteBand
	TotalKills int64
	// KillsByStop apportions the total, so a stop nobody ever stands in shows up
	// as the zero it is instead of looking like part of the plan.
	KillsByStop []int64
	// Wall is the first level where NO stop on the route pays anything, or 0 when
	// the route reaches the cap. A route with a wall is not a slow route, it is a
	// broken one, and the two must not read the same.
	Wall   int32
	Capped int32
}

// PlanRoute walks a character from `from` to its tier's cap along a route,
// taking at each level whatever the best-paying stop pays.
//
// The "best-paying stop" rule is the model, and it is worth being explicit about
// what it assumes: that a player moves to wherever pays most the moment it pays
// most, and that travel, respawn and death cost nothing. It is therefore a
// floor, never a forecast. What it does measure honestly is the SHAPE of a
// route — where the pace changes, which stop carries which stretch, and whether
// the whole thing hangs together from level 1 to the cap.
//
// One stop is a valid route, and gives exactly PlanKills.
func PlanRoute(stops []RouteStop, tier Tier, bonus ExpRewardInput, from int32) RoutePlan {
	p := RoutePlan{KillsByStop: make([]int64, len(stops)), Capped: from}
	if len(stops) == 0 {
		return p
	}
	levelCap := MaxLevelForTier(tier.ClassMaster)
	if from < 0 {
		from = 0
	}
	p.Capped = from

	for lv := from; lv < levelCap; lv++ {
		melhor, quem := int64(0), -1
		for i, s := range stops {
			if s.MobExp <= 0 || lv < s.FromLevel {
				continue
			}
			in := bonus
			in.Zone, in.MobExp, in.MobLevel = s.Zone, s.MobExp, s.MobLevel
			in.Tier, in.KillerLevel = tier, lv
			if pago := ExpReward(in); pago > melhor {
				melhor, quem = pago, i
			}
		}
		if quem < 0 {
			p.Wall = lv
			p.Bands = foldRoute(p.Steps)
			return p
		}
		span := NextLevelExpTier(lv, tier.ClassMaster) - ExpTier(lv, tier.ClassMaster)
		if span < 0 {
			span = 0
		}
		kills := span / melhor
		if span%melhor != 0 {
			kills++
		}
		p.Steps = append(p.Steps, RouteStep{
			Level: lv, Stop: quem, ExpPerKill: melhor, ExpSpan: span, Kills: kills,
		})
		p.TotalKills += kills
		p.KillsByStop[quem] += kills
		p.Capped = lv + 1
	}
	p.Bands = foldRoute(p.Steps)
	return p
}

// foldRoute collapses consecutive levels that shared a stop. The bands are the
// readable form: "1 a 96 no Deserto, 97 a 240 na Água" is a route somebody can
// follow, and four hundred rows is not.
func foldRoute(steps []RouteStep) []RouteBand {
	var out []RouteBand
	for _, s := range steps {
		if n := len(out); n > 0 && out[n-1].Stop == s.Stop && out[n-1].To == s.Level-1 {
			out[n-1].To = s.Level
			out[n-1].Kills += s.Kills
			continue
		}
		out = append(out, RouteBand{Stop: s.Stop, From: s.Level, To: s.Level, Kills: s.Kills})
	}
	return out
}
