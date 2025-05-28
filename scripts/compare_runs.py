#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
compare_runs.py  сравнивает несколько прогонов симуляции.

Принимает 1...N директорий, в каждой должны лежать результаты симуляций *.csv.
Строит графики:
    - fairness_compare.png - Jain-fairness(t)
    - cv_compare.png       - коэффициент вариации CV(t)
    - drops_compare.png    - динамика отказов на бин BIN
Пример запуска:
    python3 compare_runs.py out/exp1/csv out/exp2/csv -b 2 -o results/plots
"""

import os, argparse, warnings, pathlib
import numpy as np
import pandas as pd
import seaborn as sns
import matplotlib.pyplot as plt

# ──────────────────────── CLI ──────────────────────────────────────────────
p = argparse.ArgumentParser()
p.add_argument("runs", nargs="+",
               help="пути к каталогам с CSV-файлами (snapshots.csv, servers.csv...)")
p.add_argument("-b", "--bin", type=float, default=1.0,
               help="шаг агрегации по времени, сек (default: 1)")
p.add_argument("-o", "--out-dir", default="./plots",
               help="куда сохранять PNG-графики (default: текущая папка)")
args = p.parse_args()

BIN  = args.bin
OUT  = os.path.abspath(args.out_dir)
os.makedirs(OUT, exist_ok=True)

warnings.filterwarnings("ignore", category=FutureWarning)
sns.set_theme(style="whitegrid")

# ──────────────────────── helpers ──────────────────────────────────────────
def load(csv_dir: str, name: str) -> pd.DataFrame:
    return pd.read_csv(os.path.join(csv_dir, name))

def jain_and_cv(csv_dir: str) -> tuple[pd.Series, pd.Series]:
    """
    Возвращает две Series (fairness, cv) c индексом bin-time.
    """
    snaps   = load(csv_dir, "snapshots.csv")   # time_s, server_id, connections
    servers = load(csv_dir, "servers.csv")     # id, max_conn
    max_conn = servers.set_index("id")["max_conn"]

    snaps["util"] = snaps.connections / snaps.server_id.map(max_conn)
    snaps["bin"]  = (snaps.time_s // BIN) * BIN

    fair, cv = {}, {}
    for t, grp in snaps.groupby("bin"):
        u = grp.util.values
        if len(u) == 0:             # safety
            continue
        fair[t] = (u.sum()**2) / (len(u) * (u**2).sum())
        cv[t]   = u.std(ddof=0) / u.mean() if u.mean() else np.nan
    return (pd.Series(fair).sort_index(),
            pd.Series(cv).sort_index())

def drops_series(csv_dir: str) -> pd.Series | None:
    path = os.path.join(csv_dir, "drops.csv")
    if not os.path.exists(path):
        return None
    drops = pd.read_csv(path)
    drops["bin"] = (drops.time_s // BIN) * BIN
    s = drops.groupby("bin").size()
    return s.sort_index()

def redirects_series(csv_dir: str) -> pd.Series | None:
    path = os.path.join(csv_dir, "redirects.csv")
    if not os.path.exists(path):
        return None
    red = pd.read_csv(path)
    red["bin"] = (red.time_s // BIN) * BIN
    return red.groupby("bin").size().sort_index()

# ──────────────────────── обход всех прогонов ──────────────────────────────
fair_dict, cv_dict, drops_dict, total_drops, redirects_dict, total_redirects, rtt_dict = {}, {}, {}, {}, {}, {}, {}
for run in args.runs:
    lbl = pathlib.Path(run.rstrip("/\\")).name
    fairness, cv = jain_and_cv(run)
    fair_dict[lbl] = fairness
    cv_dict[lbl]   = cv

    s_drop = drops_series(run)
    if s_drop is not None:
        drops_dict[lbl]      = s_drop
        total_drops[lbl]     = int(s_drop.sum())
    else:
        print(f"[info] {lbl}: drops.csv не найден")

    s_red = redirects_series(run)
    if s_red is not None:
        redirects_dict[lbl]  = s_red
        total_redirects[lbl] = int(s_red.sum())
    else:
        print(f"[info] {lbl}: redirects.csv не найден")
    req = load(run, "requests.csv")           # есть всегда
    rtt_dict[lbl] = req.duration.values

# ──────────────────────── 1. Jain fairness(t) ──────────────────────────────
plt.figure(figsize=(12, 6))
for lbl, series in fair_dict.items():
    sns.lineplot(x=series.index, y=series.values, label=lbl)
plt.ylim(0, 1)
plt.ylabel("Jain fairness J(t)")
plt.xlabel("time (s)")
plt.title("Сравнение прогонов: Jain fairness")
plt.legend(title="run")
plt.tight_layout()
plt.savefig(os.path.join(OUT, "fairness_compare.png"))
plt.close()

# ──────────────────────── 2. CV(t) ─────────────────────────────────────────
plt.figure(figsize=(12, 6))
for lbl, series in cv_dict.items():
    sns.lineplot(x=series.index, y=series.values, label=lbl)
plt.ylabel("CV(t)")
plt.xlabel("time (s)")
plt.title("Сравнение прогонов: коэффициент вариации CV")
plt.legend(title="run")
plt.tight_layout()
plt.savefig(os.path.join(OUT, "cv_compare.png"))
plt.close()

# ──────────────────────── 3. Drops(t) ──────────────────────────────────────
if drops_dict:
    plt.figure(figsize=(12, 6))
    for lbl, series in drops_dict.items():
        sns.lineplot(x=series.index, y=series.values, label=lbl)
    plt.ylabel(f"drops per {BIN:.0f}s bin")
    plt.xlabel("time (s)")
    plt.title("Отказы (drops) во времени")
    plt.legend(title="run")
    plt.tight_layout()
    plt.savefig(os.path.join(OUT, "drops_compare.png"))
    plt.close()

# ──────────────────────── 4. Redirects(t) ─────────────────────────────────
if redirects_dict:
    # динамика
    plt.figure(figsize=(12, 6))
    for lbl, series in redirects_dict.items():
        sns.lineplot(x=series.index, y=series.values, label=lbl)
    plt.ylabel(f"redirects per {BIN:.0f}s bin")
    plt.xlabel("time (s)")
    plt.title("Переключения (redirects) во времени")
    plt.legend(title="run")
    plt.tight_layout()
    plt.savefig(os.path.join(OUT, "redirects_compare.png"))
    plt.close()

    # суммарно по прогонам
    plt.figure(figsize=(8, 5))
    runs = list(total_redirects.keys())
    vals = [total_redirects[r] for r in runs]
    sns.barplot(x=runs, y=vals)
    plt.ylabel("total redirects")
    plt.title("Суммарные redirect-ы по прогонам")
    plt.xticks(rotation=45, ha="right")
    plt.tight_layout()
    plt.savefig(os.path.join(OUT, "redirects_total_bar.png"))
    plt.close()

# ──────────────────────── 5. Распределение RTT per strategy ───────────────
plt.figure(figsize=(12, 6))
for lbl, arr in rtt_dict.items():
    sns.histplot(arr, bins=int(BIN), stat="density",
                 kde=True, element="step", fill=False, label=lbl)
plt.xlabel("RTT (duration), s"); plt.ylabel("density")
plt.title("Распределение RTT по стратегиям")
plt.legend(title="run"); plt.tight_layout()
plt.savefig(os.path.join(OUT, "rtt_distribution_compare.png")); plt.close()

print(f"PNG-файлы сохранены в {OUT}")
