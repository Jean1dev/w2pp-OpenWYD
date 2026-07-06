-- 0006_default_npc_seed — seeds the curated "default NPC" roster (issue #29,
-- docs/migration/npc-default-roster.md) directly into npc_definition /
-- npc_shop_item, so it is already populated on first boot (whichever service
-- runs store.Migrate first) instead of requiring a manual `dbserver
-- import-npcs` run. Moderators can edit/disable/reposition any of these via
-- the web portal (NpcAdminService) right away.
--
-- Generated from the real Release/ content tree using the same decode path
-- (buildNPCDefinitions + savefmt.DecodeMob) as `dbserver import-npcs`, filtered
-- to dbserver/internal/npcroster.Default — so the two mechanisms can never
-- disagree on slugs; re-running `import-npcs` afterward is a no-op here
-- (ON CONFLICT DO NOTHING on slug/ (npc_id,slot)).
--
-- 80 NPC definitions, 907 shop-item rows.


INSERT INTO npc_definition (slug, template_name, display_name, enabled, map_id, pos_x, pos_y, route_type, merchant) VALUES
    ('Acessorios-6064', 'Acessorios', 'Acessorios', TRUE, 0, 2095, 2095, 2, 1),
    ('Aki-1834', 'Aki', 'Aki', TRUE, 0, 1309, 312, 2, 1),
    ('Aki-3406', 'Aki', 'Aki', TRUE, 0, 2144, 2088, 2, 1),
    ('Angela-3380', 'Angela', 'Angela', TRUE, 0, 1048, 1744, 2, 2),
    ('Arma_Arch-6070', 'Arma_Arch', 'Arma_Arch', TRUE, 0, 2090, 2107, 2, 1),
    ('Arma_Mortal-6072', 'Arma_Mortal', 'Arma_Mortal', TRUE, 0, 2091, 2107, 2, 1),
    ('Arma_SemiDeus-6099', 'Arma_SemiDeus', 'Arma_SemiDeus', TRUE, 0, 2088, 2107, 2, 1),
    ('Arnod-3426', 'Arnod', 'Arnod', TRUE, 0, 2075, 2120, 2, 1),
    ('Balmers-271', 'Balmers', 'Balmers', TRUE, 0, 1305, 351, 2, 1),
    ('Bardes-2789', 'Bardes', 'Bardes', TRUE, 0, 1689, 1840, 6, 1),
    ('Berlund-3378', 'Berlund', 'Berlund', TRUE, 0, 1055, 1739, 2, 1),
    ('C._de_Montaria-3429', 'C._de_Montaria', 'C._de_Montaria', TRUE, 0, 2466, 2018, 2, 1),
    ('Cap.Cavaleiros-3416', 'Cap.Cavaleiros', 'Cap.Cavaleiros', TRUE, 0, 2144, 2120, 2, 19),
    ('Cap.Cavaleiros-5991', 'Cap.Cavaleiros', 'Cap.Cavaleiros', TRUE, 0, 2095, 2047, 2, 19),
    ('Cap_Rowena-6062', 'Cap_Rowena', 'Cap_Rowena', TRUE, 0, 1053, 1729, 2, 1),
    ('Creta-4766', 'Creta', 'Creta', TRUE, 0, 3266, 1708, 2, 1),
    ('DonatesBars-6100', 'DonatesBars', 'DonatesBars', TRUE, 0, 2514, 1709, 2, 1),
    ('Entradas-6079', 'Entradas', 'Entradas', TRUE, 0, 2085, 2097, 2, 1),
    ('Evento-6075', 'Evento', 'Evento', TRUE, 0, 2093, 2107, 2, 1),
    ('Evolucao-6071', 'Evolucao', 'Evolucao', TRUE, 0, 2085, 2101, 2, 1),
    ('Fadas-6080', 'Fadas', 'Fadas', TRUE, 0, 2085, 2105, 2, 1),
    ('Ferreiro-3414', 'Ferreiro', 'Ferreiro', TRUE, 0, 2145, 2115, 2, 1),
    ('Foema_Ancian-3407', 'Foema_Ancian', 'Foema_Ancian', TRUE, 0, 2094, 2126, 2, 19),
    ('Foema_Ancian-5993', 'Foema_Ancian', 'Foema_Ancian', TRUE, 0, 2097, 2038, 2, 19),
    ('ForeLearner-3417', 'ForeLearner', 'ForeLearner', TRUE, 0, 2130, 2125, 2, 19),
    ('ForeLearner-5992', 'ForeLearner', 'ForeLearner', TRUE, 0, 2090, 2047, 2, 19),
    ('Galford-3405', 'Galford', 'Galford', TRUE, 0, 2078, 2099, 2, 1),
    ('Gold-6076', 'Gold', 'Gold', TRUE, 0, 2094, 2095, 0, 1),
    ('Guarda_Carga-3408', 'Guarda_Carga', 'Guarda_Carga', TRUE, 0, 2144, 2082, 2, 2),
    ('Guarda_Carga-3413', 'Guarda_Carga', 'Guarda_Carga', TRUE, 0, 2102, 2116, 2, 2),
    ('Guarda_Carga-3806', 'Guarda_Carga', 'Guarda_Carga', TRUE, 0, 3660, 3135, 2, 2),
    ('Guarda_Carga-4226', 'Guarda_Carga', 'Guarda_Carga', TRUE, 0, 3241, 1683, 2, 2),
    ('Guarda_Carga-4227', 'Guarda_Carga', 'Guarda_Carga', TRUE, 0, 2104, 2050, 2, 2),
    ('Guarda_Carga-4228', 'Guarda_Carga', 'Guarda_Carga', TRUE, 0, 2190, 2148, 2, 2),
    ('Guarda_Carga-962', 'Guarda_Carga', 'Guarda_Carga', TRUE, 0, 2476, 1688, 2, 2),
    ('Guarda_Carga-974', 'Guarda_Carga', 'Guarda_Carga', TRUE, 0, 2595, 1724, 2, 2),
    ('Hestia-3379', 'Hestia', 'Hestia', TRUE, 0, 1062, 1739, 2, 1),
    ('Irena_-289', 'Irena_', 'Irena_', TRUE, 0, 1099, 319, 2, 1),
    ('Lainy-286', 'Lainy', 'Lainy', TRUE, 0, 1068, 306, 2, 1),
    ('Lubien-3437', 'Lubien', 'Lubien', TRUE, 0, 2470, 1993, 2, 1),
    ('Lucy-967', 'Lucy', 'Lucy', TRUE, 0, 2550, 1716, 2, 1),
    ('Martin-270', 'Martin', 'Martin', TRUE, 0, 1317, 346, 2, 1),
    ('Martin-3425', 'Martin', 'Martin', TRUE, 0, 2116, 2150, 2, 1),
    ('Merc_Carbunkle-992', 'Merc_Carbunkle', 'Merc_Carbunkle', TRUE, 0, 866, 999, 1, 1),
    ('Merc_Carbunkle-994', 'Merc_Carbunkle', 'Merc_Carbunkle', TRUE, 0, 1114, 1252, 1, 1),
    ('Merc_Fantasma-4511', 'Merc_Fantasma', 'Merc_Fantasma', TRUE, 0, 1073, 1715, 2, 1),
    ('Merc_Zakum-991', 'Merc_Zakum', 'Merc_Zakum', TRUE, 0, 866, 999, 1, 1),
    ('Merc_Zakum-993', 'Merc_Zakum', 'Merc_Zakum', TRUE, 0, 866, 999, 1, 1),
    ('Mestre_Archi-3427', 'Mestre_Archi', 'Mestre_Archi', TRUE, 0, 2077, 2123, 2, 19),
    ('Mestre_Archi-5994', 'Mestre_Archi', 'Mestre_Archi', TRUE, 0, 2102, 2038, 2, 19),
    ('Montarias-6073', 'Montarias', 'Montarias', TRUE, 0, 2087, 2095, 2, 1),
    ('Naomi-273', 'Naomi', 'Naomi', TRUE, 0, 1315, 329, 2, 1),
    ('Neptus-3435', 'Neptus', 'Neptus', TRUE, 0, 2467, 1999, 2, 1),
    ('Parche-3432', 'Parche', 'Parche', TRUE, 0, 2470, 2008, 2, 1),
    ('Perzen-3442', 'Perzen', 'Perzen', TRUE, 0, 1054, 1721, 0, 100),
    ('Perzen_Arcano-3441', 'Perzen_Arcano', 'Perzen_Arcano', TRUE, 0, 2480, 1705, 0, 100),
    ('Perzen_Mistico-3443', 'Perzen_Mistico', 'Perzen_Mistico', TRUE, 0, 2133, 2080, 0, 100),
    ('Perzen_Normal-3440', 'Perzen_Normal', 'Perzen_Normal', TRUE, 0, 2467, 2010, 0, 100),
    ('Pescador-5999', 'Pescador', 'Pescador', TRUE, 0, 1053, 1717, 2, 1),
    ('Prona-22', 'Prona', 'Prona', TRUE, 0, 4000, 4000, 2, 1),
    ('Prona-4232', 'Prona', 'Prona', TRUE, 0, 2598, 1725, 2, 1),
    ('Rainy-3418', 'Rainy', 'Rainy', TRUE, 0, 2133, 2124, 2, 1),
    ('Rapein-3415', 'Rapein', 'Rapein', TRUE, 0, 2090, 2126, 2, 1),
    ('Redmiron-2849', 'Redmiron', 'Redmiron', TRUE, 0, 1689, 1616, 2, 1),
    ('Reimers-287', 'Reimers', 'Reimers', TRUE, 0, 1075, 300, 2, 1),
    ('Reimers-3436', 'Reimers', 'Reimers', TRUE, 0, 2468, 1996, 2, 1),
    ('RoPerion-288', 'RoPerion', 'RoPerion', TRUE, 0, 1094, 313, 2, 1),
    ('Rophelion-3434', 'Rophelion', 'Rophelion', TRUE, 0, 2468, 2002, 2, 1),
    ('Rubyen-272', 'Rubyen', 'Rubyen', TRUE, 0, 1302, 333, 2, 1),
    ('Runas_Joias-6069', 'Runas_Joias', 'Runas_Joias', TRUE, 0, 2092, 2095, 2, 1),
    ('Ruti-961', 'Ruti', 'Ruti', TRUE, 0, 2486, 1702, 2, 1),
    ('Set_BM-6083', 'Set_BM', 'Set_BM', TRUE, 0, 2092, 2090, 2, 1),
    ('Set_FM-6084', 'Set_FM', 'Set_FM', TRUE, 0, 2094, 2090, 2, 1),
    ('Set_HT-6085', 'Set_HT', 'Set_HT', TRUE, 0, 2096, 2090, 2, 1),
    ('Set_TK-6086', 'Set_TK', 'Set_TK', TRUE, 0, 2098, 2090, 2, 1),
    ('Smith-291', 'Smith', 'Smith', TRUE, 0, 1113, 332, 2, 1),
    ('Tintas-6074', 'Tintas', 'Tintas', TRUE, 0, 2089, 2095, 2, 1),
    ('Torcedor-5995', 'Torcedor', 'Torcedor', TRUE, 0, 1068, 1719, 2, 1),
    ('Trajes-6078', 'Trajes', 'Trajes', TRUE, 0, 2104, 2107, 2, 1),
    ('Trajes2-6068', 'Trajes2', 'Trajes2', TRUE, 0, 2097, 2107, 2, 1)
ON CONFLICT (slug) DO NOTHING;

INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 591, 43, 253, 0, 0, 0, 253 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 592, 43, 253, 0, 0, 0, 253 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 593, 43, 253, 0, 0, 0, 253 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 594, 43, 253, 0, 0, 0, 253 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 595, 43, 253, 0, 0, 0, 253 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 633, 69, 20, 43, 9, 0, 0 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 633, 70, 20, 43, 9, 0, 0 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 633, 71, 50, 71, 50, 43, 9 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 633, 43, 253, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 661, 43, 9, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 662, 43, 9, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 663, 43, 9, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 633, 43, 9, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 3464, 43, 9, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 3464, 69, 20, 43, 9, 0, 0 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 3464, 70, 20, 43, 9, 0, 0 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 3464, 71, 50, 71, 50, 43, 9 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 3502, 43, 9, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 769, 2, 30, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 753, 43, 9, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 1726, 43, 9, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 3507, 3, 70, 60, 36, 43, 253 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 26, 3507, 3, 70, 2, 81, 43, 253 FROM npc_definition WHERE slug = 'Acessorios-6064'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 3322, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3323, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 699, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 410, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 4141, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 669, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 670, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 671, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 668, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 3338, 43, 9, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 4038, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 4039, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 4040, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 4041, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 4043, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 4111, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 4112, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 4113, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 646, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 647, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 693, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 694, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 695, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 1774, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 3140, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 412, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 26, 413, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-1834'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 3322, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3323, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 699, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 410, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 4141, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 669, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 670, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 671, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 668, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 3338, 43, 9, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 4038, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 4039, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 4040, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 4041, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 4043, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 4111, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 4112, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 4113, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 646, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 647, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 693, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 694, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 695, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 1774, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 3140, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 412, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 26, 413, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Aki-3406'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 2491, 2, 81, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Arma_Arch-6070'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 2731, 2, 81, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Arma_Arch-6070'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 2895, 2, 81, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Arma_Arch-6070'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 2935, 2, 81, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Arma_Arch-6070'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 2551, 2, 81, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Arma_Arch-6070'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 2611, 2, 81, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Arma_Arch-6070'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 2791, 2, 81, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Arma_Arch-6070'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 1711, 3, 99, 4, 120, 43, 253 FROM npc_definition WHERE slug = 'Arma_Arch-6070'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 2859, 60, 36, 43, 253, 0, 0 FROM npc_definition WHERE slug = 'Arma_Arch-6070'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 2863, 60, 36, 43, 253, 0, 0 FROM npc_definition WHERE slug = 'Arma_Arch-6070'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 2671, 60, 36, 43, 253, 0, 0 FROM npc_definition WHERE slug = 'Arma_Arch-6070'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 3602, 2, 81, 43, 251, 0, 0 FROM npc_definition WHERE slug = 'Arma_Mortal-6072'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3682, 2, 81, 43, 251, 0, 0 FROM npc_definition WHERE slug = 'Arma_Mortal-6072'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 3782, 2, 81, 43, 251, 0, 0 FROM npc_definition WHERE slug = 'Arma_Mortal-6072'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 3762, 2, 81, 43, 251, 0, 0 FROM npc_definition WHERE slug = 'Arma_Mortal-6072'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 3622, 2, 81, 43, 251, 0, 0 FROM npc_definition WHERE slug = 'Arma_Mortal-6072'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 3642, 2, 81, 43, 251, 0, 0 FROM npc_definition WHERE slug = 'Arma_Mortal-6072'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 3702, 2, 81, 43, 251, 0, 0 FROM npc_definition WHERE slug = 'Arma_Mortal-6072'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 1712, 3, 99, 4, 120, 43, 253 FROM npc_definition WHERE slug = 'Arma_Mortal-6072'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 3726, 60, 32, 43, 253, 0, 0 FROM npc_definition WHERE slug = 'Arma_Mortal-6072'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 3662, 60, 32, 43, 253, 0, 0 FROM npc_definition WHERE slug = 'Arma_Mortal-6072'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 3722, 60, 32, 43, 253, 0, 0 FROM npc_definition WHERE slug = 'Arma_Mortal-6072'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 3646, 2, 81, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Arma_SemiDeus-6099'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3766, 2, 81, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Arma_SemiDeus-6099'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 3606, 2, 81, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Arma_SemiDeus-6099'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 3686, 2, 81, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Arma_SemiDeus-6099'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 3626, 2, 81, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Arma_SemiDeus-6099'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 3786, 2, 81, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Arma_SemiDeus-6099'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 3706, 2, 81, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Arma_SemiDeus-6099'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 1713, 3, 99, 4, 120, 43, 253 FROM npc_definition WHERE slug = 'Arma_SemiDeus-6099'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 3665, 60, 36, 43, 253, 0, 0 FROM npc_definition WHERE slug = 'Arma_SemiDeus-6099'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 3732, 60, 36, 43, 253, 0, 0 FROM npc_definition WHERE slug = 'Arma_SemiDeus-6099'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 3736, 60, 36, 43, 253, 0, 0 FROM npc_definition WHERE slug = 'Arma_SemiDeus-6099'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 1510, 2, 30, 3, 30, 43, 250 FROM npc_definition WHERE slug = 'Arnod-3426'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 1511, 2, 30, 3, 30, 43, 250 FROM npc_definition WHERE slug = 'Arnod-3426'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 1512, 2, 30, 3, 30, 43, 250 FROM npc_definition WHERE slug = 'Arnod-3426'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 1513, 2, 30, 3, 30, 43, 250 FROM npc_definition WHERE slug = 'Arnod-3426'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 1514, 2, 30, 3, 30, 43, 250 FROM npc_definition WHERE slug = 'Arnod-3426'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 1510, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Arnod-3426'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 1511, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Arnod-3426'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 1512, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Arnod-3426'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 1513, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Arnod-3426'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 1514, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Arnod-3426'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 578, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Arnod-3426'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 1511, 2, 30, 71, 70, 43, 250 FROM npc_definition WHERE slug = 'Arnod-3426'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 1512, 2, 30, 71, 70, 43, 250 FROM npc_definition WHERE slug = 'Arnod-3426'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 1513, 3, 30, 54, 18, 43, 250 FROM npc_definition WHERE slug = 'Arnod-3426'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 1514, 2, 30, 74, 18, 43, 250 FROM npc_definition WHERE slug = 'Arnod-3426'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 3322, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Bardes-2789'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3323, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Bardes-2789'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 410, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Bardes-2789'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 411, 61, 10, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Bardes-2789'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 699, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Bardes-2789'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 776, 61, 10, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Bardes-2789'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 700, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Bardes-2789'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 697, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Bardes-2789'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 4131, 61, 10, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Bardes-2789'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 3027, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Bardes-2789'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 3028, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Bardes-2789'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 3029, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Bardes-2789'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 3030, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Bardes-2789'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 4011, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Bardes-2789'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 836, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Berlund-3378'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 881, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Berlund-3378'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 866, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Berlund-3378'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 805, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Berlund-3378'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 821, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Berlund-3378'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 851, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Berlund-3378'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 896, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Berlund-3378'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 1706, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Berlund-3378'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 2420, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'C._de_Montaria-3429'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 2421, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'C._de_Montaria-3429'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 2422, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'C._de_Montaria-3429'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 2423, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'C._de_Montaria-3429'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 2424, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'C._de_Montaria-3429'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 2425, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'C._de_Montaria-3429'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 2426, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'C._de_Montaria-3429'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 2436, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'C._de_Montaria-3429'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 2437, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'C._de_Montaria-3429'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 2438, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'C._de_Montaria-3429'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 2439, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'C._de_Montaria-3429'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 2429, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'C._de_Montaria-3429'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 2430, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'C._de_Montaria-3429'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 2427, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'C._de_Montaria-3429'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 2428, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'C._de_Montaria-3429'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 5000, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 5001, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 5002, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 5003, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 5004, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 5005, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 5006, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 5007, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 5008, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 5009, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 5010, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 5011, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 5012, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 5013, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 5014, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 5015, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 5016, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 5017, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 5018, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 5019, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 5020, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 5021, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 5022, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 5023, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-3416'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 5000, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 5001, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 5002, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 5003, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 5004, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 5005, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 5006, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 5007, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 5008, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 5009, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 5010, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 5011, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 5012, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 5013, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 5014, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 5015, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 5016, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 5017, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 5018, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 5019, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 5020, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 5021, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 5022, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 5023, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Cap.Cavaleiros-5991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 3431, 92, 10, 61, 60, 0, 0 FROM npc_definition WHERE slug = 'Cap_Rowena-6062'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 5137, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3322, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 3323, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 411, 61, 10, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 5125, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 5115, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 5111, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 5112, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 5120, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 5128, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 5119, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 5110, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 5113, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 5129, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 4127, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 5135, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 646, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 647, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Creta-4766'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 3393, 0, 0, 0, 0, 0, 253 FROM npc_definition WHERE slug = 'DonatesBars-6100'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3394, 0, 0, 0, 0, 0, 253 FROM npc_definition WHERE slug = 'DonatesBars-6100'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 3395, 0, 0, 0, 0, 0, 253 FROM npc_definition WHERE slug = 'DonatesBars-6100'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 3396, 0, 0, 0, 0, 0, 253 FROM npc_definition WHERE slug = 'DonatesBars-6100'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 3441, 0, 0, 0, 0, 0, 253 FROM npc_definition WHERE slug = 'DonatesBars-6100'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 5751, 43, 9, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'DonatesBars-6100'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 777, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Entradas-6079'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3173, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Entradas-6079'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 3182, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Entradas-6079'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 3390, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Entradas-6079'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 3391, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Entradas-6079'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 3392, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Entradas-6079'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 3324, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Entradas-6079'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 3325, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Entradas-6079'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 3326, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Entradas-6079'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 4104, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evento-6075'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 1742, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 1760, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 1761, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 1762, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 1763, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 5334, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 5335, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 5336, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 5337, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 3020, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 5338, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 669, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 670, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 671, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 4148, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 4106, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 4107, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 4108, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 4109, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 4142, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 3336, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 4140, 61, 240, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 3314, 61, 240, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 3343, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 3431, 61, 240, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 26, 4146, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Evolucao-6071'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 3900, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Fadas-6080'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3903, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Fadas-6080'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 3906, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Fadas-6080'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 3901, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Fadas-6080'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 3904, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Fadas-6080'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 3907, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Fadas-6080'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 3902, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Fadas-6080'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 3905, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Fadas-6080'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 3908, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Fadas-6080'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 3916, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Fadas-6080'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 3451, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Fadas-6080'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 3452, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Fadas-6080'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 3909, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Fadas-6080'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 26, 3910, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Fadas-6080'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 1225, 3, 30, 2, 30, 43, 250 FROM npc_definition WHERE slug = 'Ferreiro-3414'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 1226, 3, 30, 2, 30, 43, 250 FROM npc_definition WHERE slug = 'Ferreiro-3414'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 1227, 3, 30, 2, 30, 43, 250 FROM npc_definition WHERE slug = 'Ferreiro-3414'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 1228, 3, 30, 2, 30, 43, 250 FROM npc_definition WHERE slug = 'Ferreiro-3414'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 1229, 3, 30, 2, 30, 43, 250 FROM npc_definition WHERE slug = 'Ferreiro-3414'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 1225, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Ferreiro-3414'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 1226, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Ferreiro-3414'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 1227, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Ferreiro-3414'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 1228, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Ferreiro-3414'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 1229, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Ferreiro-3414'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 578, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Ferreiro-3414'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 1226, 2, 30, 71, 70, 43, 250 FROM npc_definition WHERE slug = 'Ferreiro-3414'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 1227, 2, 30, 71, 70, 43, 250 FROM npc_definition WHERE slug = 'Ferreiro-3414'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 1228, 3, 30, 54, 18, 43, 250 FROM npc_definition WHERE slug = 'Ferreiro-3414'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 1229, 2, 30, 74, 18, 43, 250 FROM npc_definition WHERE slug = 'Ferreiro-3414'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 5024, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 5027, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 5025, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 5026, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 5028, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 5029, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 5030, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 5031, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 5032, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 5033, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 5034, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 5035, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 5036, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 5037, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 5038, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 5039, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 5040, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 5041, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 5042, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 5043, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 5044, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 5045, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 5046, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 5047, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-3407'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 5024, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 5027, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 5025, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 5026, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 5028, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 5029, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 5030, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 5031, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 5032, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 5033, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 5034, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 5035, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 5036, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 5037, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 5038, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 5039, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 5040, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 5041, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 5042, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 5043, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 5044, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 5045, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 5046, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 5047, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Foema_Ancian-5993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 5072, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 5073, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 5074, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 5075, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 5076, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 5077, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 5078, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 5079, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 5080, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 5081, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 5082, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 5083, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 5084, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 5085, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 5086, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 5087, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 5088, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 5089, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 5090, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 5091, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 5092, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 5093, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 5094, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 5095, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-3417'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 5072, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 5073, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 5074, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 5075, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 5076, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 5077, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 5078, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 5079, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 5080, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 5081, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 5082, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 5083, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 5084, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 5085, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 5086, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 5087, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 5088, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 5089, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 5090, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 5091, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 5092, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 5093, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 5094, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 5095, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'ForeLearner-5992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 831, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Galford-3405'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 861, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Galford-3405'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 801, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Galford-3405'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 816, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Galford-3405'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 876, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Galford-3405'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 891, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Galford-3405'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 1701, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Galford-3405'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 4026, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 4027, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 4028, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 4029, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 4010, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 4011, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 4130, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 4129, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 4128, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 4106, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 4107, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 4108, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 4109, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 3431, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 4145, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 1739, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 3381, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 3363, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 3366, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Gold-6076'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 3322, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Hestia-3379'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3323, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Hestia-3379'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 5364, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Hestia-3379'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 5365, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Hestia-3379'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 700, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Hestia-3379'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 699, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Hestia-3379'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 776, 61, 10, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Hestia-3379'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 4030, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Hestia-3379'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 4031, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Hestia-3379'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 5373, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Hestia-3379'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 410, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Hestia-3379'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 411, 61, 10, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Hestia-3379'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 1599, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lubien-3437'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 1602, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lubien-3437'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 1605, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lubien-3437'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 1608, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lubien-3437'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 4017, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lubien-3437'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 1600, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lubien-3437'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 1603, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lubien-3437'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 1606, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lubien-3437'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 1609, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lubien-3437'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 1601, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lubien-3437'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 1604, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lubien-3437'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 1607, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lubien-3437'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 1610, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lubien-3437'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 4016, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lucy-967'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 4017, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lucy-967'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 4018, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lucy-967'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 4019, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lucy-967'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 411, 61, 10, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lucy-967'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 3386, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lucy-967'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 3387, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lucy-967'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 3388, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lucy-967'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 3389, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Lucy-967'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 3322, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3323, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 3431, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 415, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 3204, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 3381, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 3363, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 3366, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 4130, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 4129, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 4128, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 3200, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 3201, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 3202, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 3203, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 3204, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 3205, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 3206, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 3207, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 3208, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 3209, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-270'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 3322, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3323, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 3431, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 415, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 3204, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 3381, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 3363, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 3366, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 4130, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 4129, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 4128, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 3200, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 3201, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 3202, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 3203, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 3204, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 3205, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 3206, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 3207, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 3208, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 3209, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Martin-3425'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 421, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 681, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 427, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 426, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 689, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 424, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 683, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 425, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 422, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 687, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 423, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-992'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 421, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 681, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 427, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 426, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 689, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 424, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 683, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 425, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 422, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 687, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 423, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Carbunkle-994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 5489, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Fantasma-4511'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 423, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 687, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 681, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 426, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 421, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 425, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 683, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 422, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 424, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 427, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-991'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 423, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 687, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 681, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 426, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 421, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 425, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 683, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 422, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 424, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 427, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Merc_Zakum-993'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 5048, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 5049, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 5050, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 5051, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 5052, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 5053, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 5054, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 5055, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 5056, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 5057, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 5058, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 5059, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 5060, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 5061, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 5062, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 5063, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 5064, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 5065, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 5066, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 5067, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 5068, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 5069, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 5070, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 5071, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-3427'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 5048, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 5049, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 5050, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 5051, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 5052, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 5053, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 5054, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 5055, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 5056, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 5057, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 5058, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 5059, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 5060, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 5061, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 5062, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 5063, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 5064, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 5065, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 5066, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 5067, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 5068, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 5069, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 5070, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 5071, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Mestre_Archi-5994'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 2360, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 2361, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 2362, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 2363, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 2365, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 2366, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 2367, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 2368, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 2369, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 2370, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 2380, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 2372, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 2373, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 2374, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 2375, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 2376, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 2377, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 2378, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 2379, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 2381, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 2382, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 2383, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 2384, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 2385, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 2386, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 2387, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 26, 2388, 100, 100, 120, 120, 100, 0 FROM npc_definition WHERE slug = 'Montarias-6073'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 1272, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Neptus-3435'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 1284, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Neptus-3435'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 1296, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Neptus-3435'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 1308, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Neptus-3435'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 4017, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Neptus-3435'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 1273, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Neptus-3435'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 1285, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Neptus-3435'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 1297, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Neptus-3435'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 1309, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Neptus-3435'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 1274, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Neptus-3435'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 1286, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Neptus-3435'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 1298, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Neptus-3435'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 1310, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Neptus-3435'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 3322, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3323, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 681, 61, 10, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 687, 61, 10, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 4017, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 410, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 411, 61, 10, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 2300, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 2390, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 2420, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 2441, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 2442, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 2443, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 2444, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 3432, 61, 100, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 3433, 61, 100, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 3434, 61, 100, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 3435, 61, 100, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 3436, 61, 100, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 3437, 61, 100, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Parche-3432'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 4130, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Perzen_Arcano-3441'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3987, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Perzen_Arcano-3441'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 4129, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Perzen_Mistico-3443'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3988, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Perzen_Mistico-3443'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 4128, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Perzen_Normal-3440'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3987, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Perzen_Normal-3440'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 5953, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Pescador-5999'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 5954, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Pescador-5999'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 1660, 3, 30, 2, 30, 43, 250 FROM npc_definition WHERE slug = 'Rainy-3418'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 1661, 3, 30, 2, 30, 43, 250 FROM npc_definition WHERE slug = 'Rainy-3418'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 1662, 3, 30, 2, 30, 43, 250 FROM npc_definition WHERE slug = 'Rainy-3418'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 1663, 3, 30, 2, 30, 43, 250 FROM npc_definition WHERE slug = 'Rainy-3418'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 1664, 3, 30, 2, 30, 43, 250 FROM npc_definition WHERE slug = 'Rainy-3418'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 1660, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Rainy-3418'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 1661, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Rainy-3418'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 1662, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Rainy-3418'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 1663, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Rainy-3418'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 1664, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Rainy-3418'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 578, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Rainy-3418'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 1661, 2, 30, 71, 70, 43, 250 FROM npc_definition WHERE slug = 'Rainy-3418'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 1662, 2, 30, 71, 70, 43, 250 FROM npc_definition WHERE slug = 'Rainy-3418'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 1663, 3, 30, 54, 18, 43, 250 FROM npc_definition WHERE slug = 'Rainy-3418'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 1664, 2, 30, 74, 18, 43, 250 FROM npc_definition WHERE slug = 'Rainy-3418'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 1360, 2, 30, 3, 30, 43, 250 FROM npc_definition WHERE slug = 'Rapein-3415'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 1361, 2, 30, 3, 30, 43, 250 FROM npc_definition WHERE slug = 'Rapein-3415'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 1362, 2, 30, 3, 30, 43, 250 FROM npc_definition WHERE slug = 'Rapein-3415'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 1363, 2, 30, 3, 30, 43, 250 FROM npc_definition WHERE slug = 'Rapein-3415'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 1364, 2, 30, 3, 30, 43, 250 FROM npc_definition WHERE slug = 'Rapein-3415'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 1360, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Rapein-3415'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 1361, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Rapein-3415'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 1362, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Rapein-3415'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 1363, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Rapein-3415'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 1364, 3, 30, 60, 10, 43, 250 FROM npc_definition WHERE slug = 'Rapein-3415'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 578, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Rapein-3415'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 1361, 2, 30, 71, 70, 43, 250 FROM npc_definition WHERE slug = 'Rapein-3415'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 1362, 2, 30, 71, 70, 43, 250 FROM npc_definition WHERE slug = 'Rapein-3415'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 1363, 3, 30, 54, 18, 43, 250 FROM npc_definition WHERE slug = 'Rapein-3415'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 1364, 2, 30, 74, 18, 43, 250 FROM npc_definition WHERE slug = 'Rapein-3415'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 3322, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Redmiron-2849'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3323, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Redmiron-2849'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 410, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Redmiron-2849'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 411, 61, 10, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Redmiron-2849'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 699, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Redmiron-2849'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 776, 61, 10, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Redmiron-2849'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 700, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Redmiron-2849'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 697, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Redmiron-2849'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 4131, 61, 10, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Redmiron-2849'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 3027, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Redmiron-2849'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 3028, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Redmiron-2849'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 3029, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Redmiron-2849'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 3030, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Redmiron-2849'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 4011, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Redmiron-2849'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 1449, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-287'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 1452, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-287'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 1455, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-287'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 1458, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-287'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 1450, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-287'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 1453, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-287'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 1456, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-287'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 1459, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-287'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 1451, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-287'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 1454, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-287'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 1457, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-287'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 1460, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-287'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 1449, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-3436'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 1452, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-3436'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 1455, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-3436'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 1458, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-3436'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 1450, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-3436'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 1453, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-3436'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 1456, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-3436'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 1459, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-3436'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 1451, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-3436'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 1454, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-3436'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 1457, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-3436'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 1460, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Reimers-3436'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 1122, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Rophelion-3434'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 1134, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Rophelion-3434'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 1146, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Rophelion-3434'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 1158, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Rophelion-3434'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 4017, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Rophelion-3434'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 1123, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Rophelion-3434'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 1135, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Rophelion-3434'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 1147, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Rophelion-3434'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 1159, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Rophelion-3434'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 1124, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Rophelion-3434'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 1136, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Rophelion-3434'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 1148, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Rophelion-3434'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 1160, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Rophelion-3434'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 3200, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Runas_Joias-6069'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3201, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Runas_Joias-6069'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 3202, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Runas_Joias-6069'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 3203, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Runas_Joias-6069'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 3204, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Runas_Joias-6069'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 3205, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Runas_Joias-6069'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 3206, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Runas_Joias-6069'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 3207, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Runas_Joias-6069'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 3208, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Runas_Joias-6069'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 3209, 61, 255, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Runas_Joias-6069'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 5113, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Runas_Joias-6069'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 5129, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Runas_Joias-6069'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 5112, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Runas_Joias-6069'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 5110, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Runas_Joias-6069'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 5135, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Runas_Joias-6069'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 3322, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Ruti-961'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3323, 61, 120, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Ruti-961'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 700, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Ruti-961'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 410, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Ruti-961'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 411, 61, 10, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Ruti-961'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 699, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Ruti-961'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 776, 61, 10, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Ruti-961'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 1515, 60, 14, 3, 70, 43, 253 FROM npc_definition WHERE slug = 'Set_BM-6083'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 1516, 60, 14, 3, 70, 43, 253 FROM npc_definition WHERE slug = 'Set_BM-6083'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 1517, 60, 14, 3, 70, 43, 253 FROM npc_definition WHERE slug = 'Set_BM-6083'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 1518, 60, 14, 3, 70, 43, 253 FROM npc_definition WHERE slug = 'Set_BM-6083'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 1515, 2, 42, 3, 70, 43, 251 FROM npc_definition WHERE slug = 'Set_BM-6083'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 1516, 2, 42, 3, 70, 43, 251 FROM npc_definition WHERE slug = 'Set_BM-6083'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 1517, 2, 42, 3, 70, 43, 251 FROM npc_definition WHERE slug = 'Set_BM-6083'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 1518, 2, 42, 3, 70, 43, 251 FROM npc_definition WHERE slug = 'Set_BM-6083'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 1515, 2, 42, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Set_BM-6083'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 1516, 2, 42, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Set_BM-6083'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 1517, 54, 18, 72, 42, 43, 251 FROM npc_definition WHERE slug = 'Set_BM-6083'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 1518, 2, 42, 74, 18, 43, 251 FROM npc_definition WHERE slug = 'Set_BM-6083'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 1519, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Set_BM-6083'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 1520, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Set_BM-6083'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 1521, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Set_BM-6083'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 1522, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Set_BM-6083'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 1365, 60, 14, 3, 70, 43, 253 FROM npc_definition WHERE slug = 'Set_FM-6084'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 1366, 60, 14, 3, 70, 43, 253 FROM npc_definition WHERE slug = 'Set_FM-6084'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 1367, 60, 14, 3, 70, 43, 253 FROM npc_definition WHERE slug = 'Set_FM-6084'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 1368, 60, 14, 3, 70, 43, 253 FROM npc_definition WHERE slug = 'Set_FM-6084'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 1365, 2, 42, 3, 70, 43, 251 FROM npc_definition WHERE slug = 'Set_FM-6084'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 1366, 2, 42, 3, 70, 43, 251 FROM npc_definition WHERE slug = 'Set_FM-6084'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 1367, 2, 42, 3, 70, 43, 251 FROM npc_definition WHERE slug = 'Set_FM-6084'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 1368, 2, 42, 3, 70, 43, 251 FROM npc_definition WHERE slug = 'Set_FM-6084'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 1365, 2, 42, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Set_FM-6084'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 1366, 2, 42, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Set_FM-6084'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 1367, 54, 18, 72, 42, 43, 251 FROM npc_definition WHERE slug = 'Set_FM-6084'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 1368, 2, 42, 74, 18, 43, 251 FROM npc_definition WHERE slug = 'Set_FM-6084'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 1369, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Set_FM-6084'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 1370, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Set_FM-6084'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 1371, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Set_FM-6084'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 1372, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Set_FM-6084'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 1665, 60, 14, 3, 70, 43, 253 FROM npc_definition WHERE slug = 'Set_HT-6085'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 1666, 60, 14, 3, 70, 43, 253 FROM npc_definition WHERE slug = 'Set_HT-6085'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 1667, 60, 14, 3, 70, 43, 253 FROM npc_definition WHERE slug = 'Set_HT-6085'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 1668, 60, 14, 3, 70, 43, 253 FROM npc_definition WHERE slug = 'Set_HT-6085'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 1665, 2, 42, 3, 70, 43, 251 FROM npc_definition WHERE slug = 'Set_HT-6085'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 1666, 2, 42, 3, 70, 43, 251 FROM npc_definition WHERE slug = 'Set_HT-6085'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 1667, 2, 42, 3, 70, 43, 251 FROM npc_definition WHERE slug = 'Set_HT-6085'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 1668, 2, 42, 3, 70, 43, 251 FROM npc_definition WHERE slug = 'Set_HT-6085'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 1665, 2, 42, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Set_HT-6085'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 1666, 2, 42, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Set_HT-6085'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 1667, 54, 18, 72, 42, 43, 251 FROM npc_definition WHERE slug = 'Set_HT-6085'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 1668, 2, 42, 74, 18, 43, 251 FROM npc_definition WHERE slug = 'Set_HT-6085'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 1669, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Set_HT-6085'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 1670, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Set_HT-6085'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 1671, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Set_HT-6085'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 1672, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Set_HT-6085'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 1230, 60, 14, 3, 70, 43, 253 FROM npc_definition WHERE slug = 'Set_TK-6086'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 1231, 60, 14, 3, 70, 43, 253 FROM npc_definition WHERE slug = 'Set_TK-6086'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 1232, 60, 14, 3, 70, 43, 253 FROM npc_definition WHERE slug = 'Set_TK-6086'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 1233, 60, 14, 3, 70, 43, 253 FROM npc_definition WHERE slug = 'Set_TK-6086'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 1230, 2, 42, 3, 70, 43, 251 FROM npc_definition WHERE slug = 'Set_TK-6086'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 1231, 2, 42, 3, 70, 43, 251 FROM npc_definition WHERE slug = 'Set_TK-6086'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 1232, 2, 42, 3, 70, 43, 251 FROM npc_definition WHERE slug = 'Set_TK-6086'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 1233, 2, 42, 3, 70, 43, 251 FROM npc_definition WHERE slug = 'Set_TK-6086'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 1230, 2, 42, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Set_TK-6086'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 1231, 2, 42, 71, 90, 43, 251 FROM npc_definition WHERE slug = 'Set_TK-6086'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 1232, 54, 18, 72, 42, 43, 251 FROM npc_definition WHERE slug = 'Set_TK-6086'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 1233, 2, 42, 74, 18, 43, 251 FROM npc_definition WHERE slug = 'Set_TK-6086'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 1234, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Set_TK-6086'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 1235, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Set_TK-6086'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 1236, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Set_TK-6086'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 1237, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Set_TK-6086'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 1106, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Smith-291'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 1118, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Smith-291'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 1130, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Smith-291'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 1154, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Smith-291'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 4016, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Smith-291'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 1256, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Smith-291'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 1268, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Smith-291'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 1280, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Smith-291'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 1304, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Smith-291'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 3407, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 3408, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 3409, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 3410, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 3411, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 3414, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 3415, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 3416, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 3412, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 3413, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 3406, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 3405, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 3404, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 3403, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 3402, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 3401, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 3400, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 3399, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 3398, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 3397, 61, 5, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Tintas-6074'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 5965, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Torcedor-5995'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 5636, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Torcedor-5995'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 4150, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 4151, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 4152, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 4153, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 4154, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 4155, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 4156, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 4157, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 4158, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 4159, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 4160, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 4161, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 12, 4162, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 13, 4163, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 4164, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 4165, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 4166, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 4167, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 4168, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 4169, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 4170, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 4171, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 4172, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 4173, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 4174, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 4175, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 26, 4176, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes-6078'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 0, 4177, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 1, 4178, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 2, 4179, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 3, 4180, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 4, 4181, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 5, 4182, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 6, 4183, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 7, 4184, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 8, 4185, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 9, 4186, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 10, 4187, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 11, 4188, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 14, 4899, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 15, 4192, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 16, 4193, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 17, 4194, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 18, 4195, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 19, 4196, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 20, 4197, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 21, 4198, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 22, 4199, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 23, 4193, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 24, 4194, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 25, 4190, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;
INSERT INTO npc_shop_item (npc_id, slot, item_index, eff1, effv1, eff2, effv2, eff3, effv3)
    SELECT id, 26, 4191, 0, 0, 0, 0, 0, 0 FROM npc_definition WHERE slug = 'Trajes2-6068'
    ON CONFLICT (npc_id, slot) DO NOTHING;

UPDATE npc_config_meta SET version = version + 1 WHERE id = TRUE;

