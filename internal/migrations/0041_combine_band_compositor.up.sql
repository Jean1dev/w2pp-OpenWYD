-- 0041_combine_band_compositor — o compositor ganha a sua própria curva por
-- faixa de item, ao lado da curva do refino +10.
--
-- Tipos 3 e 4 são as armas e as armaduras do compositor, na mesma lógica dos
-- tipos 1 e 2 da +10. Curvas separadas porque são decisões separadas: deixar o
-- conjunto C mais fácil de refinar não é deixá-lo mais fácil de compor, e uma
-- tabela só amarraria as duas máquinas sem o moderador perceber.
--
-- A restrição criada em 0040 é inline, então o nome é o que o Postgres gera:
-- combine_band_slot_kind_check.

ALTER TABLE combine_band DROP CONSTRAINT IF EXISTS combine_band_slot_kind_check;
ALTER TABLE combine_band ADD CONSTRAINT combine_band_slot_kind_check
    CHECK (slot_kind IN (1, 2, 3, 4));
