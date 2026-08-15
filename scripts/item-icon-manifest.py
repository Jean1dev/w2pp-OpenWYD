#!/usr/bin/env python3
"""Generate the item-icon manifest the web front-end needs from the item catalog.

Reads the editable catalog (Release/Common/ItemList.csv — the same file the
tmServer/webServer load) and emits one JSON row per item with the fields a UI
needs to pick and decorate an icon: the equip slot, the grade, the inventory
grid size, and an `icon_key`.

`icon_key` is the important part. The client never sees the item index when it
draws an item: it reads the very same catalog (STRUCT_ITEMLIST) and the only
visual fields there are IndexMesh, IndexTexture and nPos (Basedef.h:1162-1185,
parsed at Basedef.cpp:5757-5772). So the image is a function of that triple,
which collapses ~3.2k named items into ~1k distinct icons — that triple is what
an icon pack should be keyed by, not the item index.

Optionally cross-checks the compiled Release/DBsrv/run/ItemList.bin, which is
6500 records of 140 bytes each, obfuscated with a flat XOR 0x5A (no header; the
file has 4 trailing bytes). Divergences mean the .bin is stale relative to the
.csv — the .csv wins, since that is what the Go services parse.

Usage:
  ./scripts/item-icon-manifest.py                       # manifest to stdout
  ./scripts/item-icon-manifest.py -o web/items.json     # manifest to a file
  ./scripts/item-icon-manifest.py --check-bin           # also diff csv vs bin
"""

from __future__ import annotations

import argparse
import json
import struct
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
CSV = REPO / "Release" / "Common" / "ItemList.csv"
BIN = REPO / "Release" / "DBsrv" / "run" / "ItemList.bin"

BIN_RECORD = 140  # Name[64] + 8 shorts + 12*(short,short) + int Price + 4 shorts
BIN_COUNT = 6500  # MAX_ITEMLIST (Basedef.h:199)
BIN_XOR = 0x5A

# nPos is a bitmask over STRUCT_MOB.Equip[16]: bit i means "goes in Equip[i]".
# 0 means "not equippable" (potions, quest items, coupons…). Bit 6|7 (192) is a
# two-handed weapon, which takes the shield slot too. Slot 13 is the fairy
# (CMob.cpp:712 keys off Equip[13] for the 39xx fairies) and 14 the mount
# (handler/character.go:662 mountEquipSlot).
SLOTS = {
    1 << 0: "face",
    1 << 1: "helmet",
    1 << 2: "armor",
    1 << 3: "pants",
    1 << 4: "gloves",
    1 << 5: "boots",
    1 << 6: "weapon",
    1 << 7: "shield",
    1 << 8: "accessory",
    1 << 9: "amulet",
    1 << 10: "orb",
    1 << 11: "gem",
    1 << 12: "medal",
    1 << 13: "fairy",
    1 << 14: "mount",
    1 << 15: "cape",
}


def slot_names(pos: int) -> list[str]:
    """Decode the nPos bitmask into slot names (empty list when pos == 0).

    nPos is a signed short, so capes (bit 15) show up as -32768 in the CSV;
    mask to 16 bits before decoding.
    """
    pos &= 0xFFFF
    if pos == 0:
        return []
    return [name for bit, name in sorted(SLOTS.items()) if pos & bit]


def display_name(raw: str) -> str:
    """Catalog names are Latin-1 with underscores for spaces (`Botas_Douradas(N)`)."""
    return raw.replace("_", " ").strip()


def parse_csv(path: Path) -> dict[int, dict]:
    """Parse ItemList.csv following BASE_ReadItemListFile (Basedef.cpp:5717).

    Columns: index, name, mesh.texture, lvl.str.int.dex.con, unique, price,
    pos, extra, grade, then up to 12 `EF_<name>,value` pairs.
    """
    items: dict[int, dict] = {}
    for line in path.read_bytes().splitlines():
        row = line.decode("latin-1").strip()
        if not row:
            continue
        f = row.split(",")
        if len(f) < 9:
            continue
        try:
            index = int(f[0])
        except ValueError:
            continue
        if index < 0 or not f[1]:
            continue
        mesh, _, texture = f[2].partition(".")
        req = (f[3].split(".") + ["0"] * 5)[:5]

        def num(s: str) -> int:
            try:
                return int(s.strip())
            except ValueError:
                return 0

        effects = {}
        for i in range(9, len(f) - 1):
            token = f[i].strip()
            if token.startswith("EF_"):
                effects[token] = num(f[i + 1])

        pos = num(f[6]) & 0xFFFF
        items[index] = {
            "index": index,
            "name": f[1],
            "display_name": display_name(f[1]),
            "mesh": num(mesh),
            "texture": num(texture),
            "pos": pos,
            "slots": slot_names(pos),
            "grade": num(f[8]),
            "price": num(f[5]),
            "req_level": num(req[0]),
            "class_mask": effects.get("EF_CLASS", 0),
            "grid": effects.get("EF_GRID", 0),
            "icon_key": f"m{num(mesh)}_t{num(texture)}_p{pos}",
        }
    return items


def parse_bin(path: Path) -> dict[int, dict]:
    """Decode the compiled ItemList.bin (XOR 0x5A, 6500 x 140 bytes, no header)."""
    raw = path.read_bytes()
    data = bytes(b ^ BIN_XOR for b in raw)
    out: dict[int, dict] = {}
    for i in range(BIN_COUNT):
        rec = data[i * BIN_RECORD : (i + 1) * BIN_RECORD]
        if len(rec) < BIN_RECORD:
            break
        name = rec[:64].split(b"\x00")[0].decode("latin-1")
        if not name:
            continue
        mesh, texture, _vfx = struct.unpack("<3h", rec[64:70])
        _unique, pos, _extra, grade = struct.unpack("<4h", rec[132:140])
        out[i] = {"name": name, "mesh": mesh, "texture": texture, "pos": pos & 0xFFFF, "grade": grade}
    return out


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("-o", "--output", type=Path, help="write the manifest here instead of stdout")
    ap.add_argument("--check-bin", action="store_true", help="diff ItemList.csv against ItemList.bin")
    args = ap.parse_args()

    items = parse_csv(CSV)
    keys = {it["icon_key"] for it in items.values()}

    manifest = {
        "source": "Release/Common/ItemList.csv",
        "item_count": len(items),
        "icon_key_count": len(keys),
        "items": [items[i] for i in sorted(items)],
    }
    blob = json.dumps(manifest, ensure_ascii=False, indent=2)
    if args.output:
        args.output.write_text(blob, encoding="utf-8")
    else:
        print(blob)

    print(f"items: {len(items)}  distinct icon_key: {len(keys)}", file=sys.stderr)

    if args.check_bin:
        if not BIN.exists():
            print(f"missing {BIN}", file=sys.stderr)
            return 1
        binrows = parse_bin(BIN)
        drift = [
            i
            for i, it in items.items()
            if i in binrows
            and (it["mesh"], it["texture"], it["pos"]) != (binrows[i]["mesh"], binrows[i]["texture"], binrows[i]["pos"])
        ]
        only_bin = sorted(set(binrows) - set(items))
        print(
            f"bin rows: {len(binrows)}  visual drift csv!=bin: {len(drift)}  only in bin: {len(only_bin)}",
            file=sys.stderr,
        )
        for i in drift[:10]:
            print(
                f"  #{i} {items[i]['name']}: csv mesh/tex/pos="
                f"{items[i]['mesh']}/{items[i]['texture']}/{items[i]['pos']} "
                f"bin={binrows[i]['mesh']}/{binrows[i]['texture']}/{binrows[i]['pos']}",
                file=sys.stderr,
            )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
