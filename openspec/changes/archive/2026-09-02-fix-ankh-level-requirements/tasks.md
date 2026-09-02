## 1. Catalog Requirements

- [x] 1.1 Update items 661, 662, and 663 in `Release/Common/ItemList.csv` to raw `ReqLvl` 160 without changing their remaining requirement or item fields, and verify `TestRequirementsClasseD` reads level 160 for all three.
- [x] 1.2 Update only the decoded `ReqLvl` fields for records 661, 662, and 663 in `Release/DBsrv/run/ItemList.bin`, preserving its XOR-0x5A, 140-byte-record layout, and verify the binary remains exactly 910004 bytes.

## 2. Regression Coverage

- [x] 2.1 Add explicit real-catalog anchors for all three Ankhs to `tmserver/internal/content` tests and verify `go test ./tmserver/internal/content` passes.
- [x] 2.2 Add explicit decoded-binary anchors for all three Ankhs to `webserver/internal/itemcatalog` tests and verify `go test ./webserver/internal/itemcatalog` passes, including zero CSV/binary requirement mismatches.

## 3. Integration Verification

- [x] 3.1 Run `make test` and verify the repository test suite passes with the synchronized catalogs.
- [x] 3.2 Install the updated `ItemList.bin` in a WYD 7662 client and verify items 661, 662, and 663 each display required level 161 and a Mortal can equip them at, but not below, that displayed level.
