#!/usr/bin/env bash
# run_many.sh запускает симулятор для набора стратегий,
# подменяя поле balancer.strategy в едином YAML-шаблоне,
# а затем строит сравнительные графики через compare_runs.py.
#
# Использование:
#   ./run_many.sh [-p plots_dir] strat1 strat2 ...
#
# Требует:
#   - config/default.yaml – базовый конфиг
#   - compare_runs.py     – скрипт построения графиков
#   - исходники симулятора в cmd/sim

set -euo pipefail

# ────────────────── аргументы CLI ─────────────────────────────────────────
PLOT_DIR="./plots"

while [[ $# -gt 0 && "$1" =~ ^- ]]; do
  case "$1" in
    -p|--plots)
      PLOT_DIR="$(realpath "$2")"; shift 2 ;;
    *)
      echo "Unknown flag $1"; exit 1 ;;
  esac
done

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 [-p plots_dir] strategy1 [strategy2 ...]" >&2
  exit 1
fi
STRATS=("$@")

# ────────────────── пути и подготовка ─────────────────────────────────────
BIN_DIR="./bin";    mkdir -p "$BIN_DIR"
BIN="$BIN_DIR/sim"
BASE_CFG="./config/default.yaml"
OUT_ROOT="./csv";   mkdir -p "$OUT_ROOT"
mkdir -p "$PLOT_DIR"

# ────────────────── сборка бинаря ──────────────────────────────

echo "- building simulator -> $BIN"
go build -o "$BIN" ./cmd/sim

[[ -f "$BASE_CFG" ]] || { echo "!! $BASE_CFG not found"; exit 1; }

# ────────────────── запуск симуляций ──────────────────────────────────────
CSV_DIRS=()   # накопим каталоги для compare_runs.py

for STRAT in "${STRATS[@]}"; do
  echo "- run strategy '$STRAT'"

  TMP_CFG="$(mktemp --suffix=.yml)"
  if command -v yq >/dev/null 2>&1 ; then
    yq eval ".balancer.strategy = \"$STRAT\"" "$BASE_CFG" > "$TMP_CFG"
    echo "  -> balancer.strategy:" \
         "$(yq eval '.balancer.strategy' "$TMP_CFG")"
  else
    sed -e "s/^\\s*strategy: .*/  strategy: \"$STRAT\"/" \
        "$BASE_CFG" > "$TMP_CFG"
    echo "  -> balancer.strategy:" \
         "$(grep -m1 'strategy:' "$TMP_CFG" | awk '{print $2}')"
  fi

  OUT_DIR="$OUT_ROOT/$STRAT"; mkdir -p "$OUT_DIR"
  CSV_DIRS+=("$OUT_DIR")

  "$BIN" --cfg "$TMP_CFG" --out "$OUT_DIR"

  rm -f "$TMP_CFG"
done

# ────────────────── построение сравнительных графиков ────────────────────
echo "- build comparison plots -> $PLOT_DIR"
python3 ./scripts/compare_runs.py "${CSV_DIRS[@]}" -o "$PLOT_DIR" -b 10.0

echo "Готово! CSV-файлы: $OUT_ROOT/<strategy>/ ; графики: $PLOT_DIR"
