#!/usr/bin/env bash
set -euo pipefail

out=${1:-data/scifact}
mkdir -p "$out"

uv run --with ir-datasets python - "$out" <<'PY'
import json
import sys
from pathlib import Path

import ir_datasets

out = Path(sys.argv[1])
dataset = ir_datasets.load("beir/scifact")

for name, records, expected in (
    ("docs.jsonl", dataset.docs_iter(), 5_183),
    ("queries.jsonl", dataset.queries_iter(), 1_109),
):
    path = out / name
    tmp = path.with_suffix(path.suffix + ".tmp")
    with tmp.open("w", encoding="utf-8") as f:
        count = 0
        for record in records:
            f.write(json.dumps(record._asdict(), ensure_ascii=False) + "\n")
            count += 1
    if count != expected:
        tmp.unlink()
        raise RuntimeError(f"{name}: expected {expected} records, got {count}")
    tmp.replace(path)
    print(f"Wrote {count:,} records to {path}")
PY
