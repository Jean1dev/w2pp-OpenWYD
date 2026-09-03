CREATE TABLE sapphire_balance (
    singleton       BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    version         BIGINT NOT NULL DEFAULT 1,
    hekalotia_cost  INTEGER NOT NULL DEFAULT 8 CHECK (hekalotia_cost BETWEEN 4 AND 32),
    akelonia_cost   INTEGER NOT NULL DEFAULT 8 CHECK (akelonia_cost BETWEEN 4 AND 32)
);

INSERT INTO sapphire_balance (singleton) VALUES (TRUE);
