-- As faixas do compositor não cabem na restrição antiga, então saem antes dela
-- voltar. O compositor volta a correr sem curva, só com o peso dos sacrifícios.
DELETE FROM combine_band WHERE slot_kind IN (3, 4);
ALTER TABLE combine_band DROP CONSTRAINT IF EXISTS combine_band_slot_kind_check;
ALTER TABLE combine_band ADD CONSTRAINT combine_band_slot_kind_check
    CHECK (slot_kind IN (1, 2));
