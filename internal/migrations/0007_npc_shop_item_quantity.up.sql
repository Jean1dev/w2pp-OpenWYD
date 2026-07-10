-- 0007_npc_shop_item_quantity — moderator-facing stack quantity for NPC shops.
--
-- The legacy client has no separate shop quantity field. Stacks are encoded as
-- a normal STRUCT_ITEM effect pair: EF_AMOUNT (61) + cValue. Store quantity as a
-- first-class admin field, then tmServer materializes EF_AMOUNT when spawning the
-- NPC shop item.

ALTER TABLE npc_shop_item
    ADD COLUMN quantity SMALLINT NOT NULL DEFAULT 1 CHECK (quantity BETWEEN 1 AND 255);

UPDATE npc_shop_item
SET quantity = CASE
        WHEN eff1 = 61 AND effv1 BETWEEN 1 AND 255 THEN effv1
        WHEN eff2 = 61 AND effv2 BETWEEN 1 AND 255 THEN effv2
        WHEN eff3 = 61 AND effv3 BETWEEN 1 AND 255 THEN effv3
        ELSE quantity
    END;

UPDATE npc_shop_item
SET eff1 = 0, effv1 = 0
WHERE eff1 = 61;

UPDATE npc_shop_item
SET eff2 = 0, effv2 = 0
WHERE eff2 = 61;

UPDATE npc_shop_item
SET eff3 = 0, effv3 = 0
WHERE eff3 = 61;
