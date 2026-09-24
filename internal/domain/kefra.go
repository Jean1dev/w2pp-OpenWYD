package domain

// KefraState is the durable weekly boss cycle. Revision zero means no saved
// cycle exists yet; timestamps are Unix seconds scheduled by the local calendar.
type KefraState struct {
	Defeated      bool
	NextSpawnUnix int64
	LastSpawnUnix int64
	Revision      int64
}
