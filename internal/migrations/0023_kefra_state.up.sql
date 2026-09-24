CREATE TABLE kefra_state (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    defeated BOOLEAN NOT NULL,
    next_spawn_unix BIGINT NOT NULL CHECK (next_spawn_unix > 0),
    last_spawn_unix BIGINT NOT NULL CHECK (last_spawn_unix >= 0 AND last_spawn_unix < next_spawn_unix),
    revision BIGINT NOT NULL CHECK (revision > 0)
);
