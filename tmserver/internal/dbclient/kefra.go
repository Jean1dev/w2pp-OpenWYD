package dbclient

import (
	"context"
	"fmt"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// LoadKefraState loads the event independently of any player session.
func (c *Client) LoadKefraState(ctx context.Context) (world.KefraState, error) {
	resp, err := c.api.LoadKefraState(ctx, &dbv1.LoadKefraStateRequest{})
	if err != nil {
		return world.KefraState{}, fmt.Errorf("dbclient: load kefra state: %w", err)
	}
	p := resp.GetState()
	return world.KefraState{Defeated: p.GetDefeated(), NextSpawnUnix: p.GetNextSpawnUnix(), LastSpawnUnix: p.GetLastSpawnUnix(), Revision: p.GetRevision()}, nil
}

// SaveKefraState persists a value snapshot of the weekly cycle.
func (c *Client) SaveKefraState(ctx context.Context, st world.KefraState) error {
	resp, err := c.api.SaveKefraState(ctx, &dbv1.SaveKefraStateRequest{State: &dbv1.KefraState{Defeated: st.Defeated, NextSpawnUnix: st.NextSpawnUnix, LastSpawnUnix: st.LastSpawnUnix, Revision: st.Revision}})
	if err != nil {
		return fmt.Errorf("dbclient: save kefra state: %w", err)
	}
	if !resp.GetOk() {
		return fmt.Errorf("dbclient: save kefra state rejected")
	}
	return nil
}
