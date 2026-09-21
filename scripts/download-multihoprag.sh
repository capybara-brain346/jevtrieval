#!/usr/bin/env bash
set -euo pipefail

out=${1:-data/multihoprag}

hf download yixuantt/MultiHopRAG \
  --type dataset \
  --include MultiHopRAG.json \
  --include corpus.json \
  --local-dir "$out"

test -s "$out/MultiHopRAG.json"
test -s "$out/corpus.json"
printf 'Downloaded MultiHopRAG to %s\n' "$out"
