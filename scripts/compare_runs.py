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

def stickiness_series(csv_dir: str) -> pd.Series | None:
    """
    На основе requests.csv считаем для каждого запроса, был ли он обслужен 'родным' сервером.
    Ожидаем, что в requests.csv есть столбцы:
        server_id, session_id, start_s, end_s, duration

    Логика:
    1) Для каждой session_id находим сервер initial_server, 
       с которого пришёл первый запрос (минимальное start_s).
    2) Для каждого запроса помечаем stickied = 1, если server_id == initial_server, иначе 0.
    3) Биннинг по времени по полю start_s:
         bin = floor(start_s / BIN) * BIN
    4) Для каждого бина считаем долю stickied запросов:
         stickiness(t) = sum(stickied_i) / total_requests_in_bin
    """
    path = os.path.join(csv_dir, "requests.csv")
    if not os.path.exists(path):
        print(f"[info] {path} не найден, stickiness не считается")
        return None

    req = pd.read_csv(path)  # ожидание колонок: server_id, session_id, start_s, ...

    # 1) Определяем для каждой сессии initial_server (сервер первой записи по start_s)
    #    Берём минимальное start_s для каждой session_id
    first_reqs = req.loc[:, ["session_id", "start_s", "server_id"]].copy()
    first_reqs = first_reqs.sort_values(["session_id", "start_s"])
    first_reqs = first_reqs.drop_duplicates(subset=["session_id"], keep="first")
    initial_map = first_reqs.set_index("session_id")["server_id"].to_dict()

    # 2) Проставляем для каждой строки, какой initial_server у её session_id
    req["initial_server"] = req.session_id.map(initial_map)
    req["stickied"] = (req.server_id == req.initial_server).astype(int)

    # 3) Биннинг по времени по полю start_s
    req["bin"] = (req.start_s // BIN) * BIN

    # 4) Группируем по bin и вычисляем долю stickied=1
    stickiness = {}
    for t, grp in req.groupby("bin"):
        if len(grp) == 0:
            continue
        stickiness[t] = grp.stickied.sum() / len(grp)

    return pd.Series(stickiness).sort_index()

def requests_series(csv_dir: str) -> pd.Series:
    """
    Считывает requests.csv и возвращает Series: bin → число запросов в этом бине.
    Предполагается, что в requests.csv есть поле start_s.
    """
    path = os.path.join(csv_dir, "requests.csv")
    if not os.path.exists(path):
        raise FileNotFoundError(f"{path} не найден")

    req = pd.read_csv(path)  # ожидаем хотя бы столбцы: start_s, session_id, server_id, …
    req["bin"] = (req.start_s // BIN) * BIN
    s = req.groupby("bin").size()
    return s.sort_index()

# ──────────────────────── обход всех прогонов ──────────────────────────────
fair_dict, cv_dict, drops_dict, total_drops, redirects_dict, total_redirects, rtt_dict = {}, {}, {}, {}, {}, {}, {}
stickiness_dict = {}
requests_dict = {}

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
    s_stick = stickiness_series(run)
    if s_stick is not None:
        stickiness_dict[lbl] = s_stick
    req = load(run, "requests.csv")           # есть всегда
    rtt_dict[lbl] = req.duration.values

    s_req = requests_series(run)
    requests_dict[lbl] = s_req

# ──────────────────────── 1. Jain fairness(t) ──────────────────────────────
plt.figure(figsize=(12, 6))
for lbl, series in fair_dict.items():
    sns.lineplot(x=series.index, y=series.values, label=lbl)
plt.ylim(0, 1)
plt.ylabel("Индекс J(t)")
plt.xlabel("Время (с)")
# plt.title("Сравнение прогонов: Jain fairness")
plt.legend(title="run")
plt.tight_layout()
plt.savefig(os.path.join(OUT, "fairness_compare.png"))
plt.close()

# ──────────────────────── 2. CV(t) ─────────────────────────────────────────
plt.figure(figsize=(12, 6))
for lbl, series in cv_dict.items():
    sns.lineplot(x=series.index, y=series.values, label=lbl)
plt.ylabel("CV(t)")
plt.xlabel("Время (с)")
# plt.title("Сравнение прогонов: коэффициент вариации CV")
plt.legend(title="run")
plt.tight_layout()
plt.savefig(os.path.join(OUT, "cv_compare.png"))
plt.close()

# ──────────────────────── 3. Drops(t) ──────────────────────────────────────
if drops_dict:
    plt.figure(figsize=(12, 6))
    for lbl, series in drops_dict.items():
        sns.lineplot(x=series.index, y=series.values, label=lbl)
    plt.ylabel(f"Количество отказов на {BIN:.0f}с")
    plt.xlabel("Время (с)")
    # plt.title("Отказы (drops) во времени")
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
    plt.ylabel(f"Перенаправлений на {BIN:.0f}с")
    plt.xlabel("Время (с)")
    # plt.title("Переключения (redirects) во времени")
    plt.legend(title="run")
    plt.tight_layout()
    plt.savefig(os.path.join(OUT, "redirects_compare.png"))
    plt.close()

    # суммарно по прогонам
    plt.figure(figsize=(8, 5))
    runs = list(total_redirects.keys())
    vals = [total_redirects[r] for r in runs]
    sns.barplot(x=runs, y=vals)
    plt.ylabel("Суммарное кол-во перенаправлений")
    # plt.title("Суммарные redirect-ы по прогонам")
    plt.xticks(rotation=45, ha="right")
    plt.tight_layout()
    plt.savefig(os.path.join(OUT, "redirects_total_bar.png"))
    plt.close()

# ──────────────────────── 5. Распределение RTT per strategy ───────────────
plt.figure(figsize=(12, 6))

for lbl, arr in rtt_dict.items():
    arr_ms = np.asarray(arr) * 1000          # 1 секунды → миллисекунды
    sns.histplot(
        arr_ms,
        bins=80,                             # подберите шаг при желании
        stat='count',                        # 2 считаем частоты, не плотность
        element='step',                      # только контуры, без заливки
        fill=False,
        linewidth=1.4,
        label=lbl
    )

plt.xlabel("RTT, мс")
plt.ylabel("Кол-во запросов")
# plt.title("Распределение RTT по стратегиям")
plt.xlim(left=0)                             # отрицательных значений быть не может
plt.legend(title="run")
plt.tight_layout()
plt.savefig(os.path.join(OUT, "rtt_distribution_compare.png"))
plt.close()

# --- TEMP
plt.figure(figsize=(12, 6))

for lbl, arr in rtt_dict.items():
    rtt_ms = np.asarray(arr) * 1000

    # строим эмпирическую CDF
    x = np.sort(rtt_ms)
    y = np.linspace(0, 1, len(x), endpoint=False)  # F(x) = P(RTT ≤ x)

    # линия «лестницей» — чтобы ECDF была аккуратная
    plt.step(x, y, where="post", label=lbl)

# горизонтальные подсказки: медиана (0.5) и p95 (0.95)
for q, ls in zip([0.5, 0.95], [":", "--"]):
    plt.axhline(q, color="grey", linestyle=ls, linewidth=0.8)
    plt.text(plt.xlim()[1]*0.98, q+0.01, f"{int(q*100)}-й перц.", 
             ha="right", va="bottom", color="grey", fontsize=8)

plt.xlabel("RTT, мс")
plt.ylabel("Доля обслуженных запросов  ≤  RTT")
plt.xlim(left=0)
plt.ylim(0, 1)
plt.legend(title="run", loc="lower right")
plt.tight_layout()
plt.savefig(os.path.join(OUT, "rtt_ecdf_compare.png"))
plt.close()

# ──────────────────────── 6. Stickiness(t) ────────────────────────────────
if stickiness_dict:
    plt.figure(figsize=(12, 6))
    for lbl, series in stickiness_dict.items():
        sns.lineplot(x=series.index, y=series.values, label=lbl)
    plt.ylim(0, 1)
    plt.ylabel("Доля изначальных серверов при обслуживании")
    plt.xlabel("Время (с)")
    # plt.title("Доля запросов, обслуженных 'родным' сервером (stickiness)")
    plt.legend(title="run")
    plt.tight_layout()
    plt.savefig(os.path.join(OUT, "stickiness_compare.png"))
    plt.close()

# ──────────────────────── 7. Requests(t) ────────────────────────────────
if requests_dict:
    plt.figure(figsize=(12, 6))
    for lbl, series in requests_dict.items():
        sns.lineplot(x=series.index, y=series.values, label=lbl)
    plt.ylabel(f"Число запросов на{BIN:.0f}с")
    plt.xlabel("Время (с)")
    # plt.title("Сравнение прогонов: обслуженные запросы во времени")
    plt.legend(title="run")
    plt.tight_layout()
    plt.savefig(os.path.join(OUT, "requests_count_compare.png"))
    plt.close()


print(f"PNG-файлы сохранены в {OUT}")
