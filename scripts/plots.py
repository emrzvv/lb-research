#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
plots_ru.py - построение графиков из CSV-результатов симуляции (русская локализация)
Пример запуска:
    python3 plots_ru.py -c out/exp1/csv -p out/exp1/plots
"""

import os, warnings, argparse, math
import pandas as pd
import seaborn as sns
import matplotlib.pyplot as plt

# ──────────────────────── настройки шрифтов ─────────────────────────
plt.rcParams["font.family"] = "DejaVu Sans"          # поддержка кириллицы
plt.rcParams["axes.unicode_minus"] = False           # корректный минус

# ──────────────────────── аргументы CLI ─────────────────────────────
parser = argparse.ArgumentParser(
    description="Построение графиков из CSV-файлов результатов симуляции"
)
parser.add_argument(
    "-c", "--csv-dir", default="csv",
    help="директория с CSV-файлами (snapshots.csv, arrivals.csv ...)")
parser.add_argument(
    "-p", "--plot-dir", default=None,
    help="куда сохранять PNG-графики (если не указано, <csv-dir>/plots)")
args = parser.parse_args()

CSV_DIR  = args.csv_dir.rstrip("/\\")
PLOT_DIR = args.plot_dir.rstrip("/\\") if args.plot_dir else os.path.join(CSV_DIR, "plots")
os.makedirs(PLOT_DIR, exist_ok=True)

# ──────────────────────── чтение CSV ────────────────────────────────

def r(name: str) -> pd.DataFrame:
    return pd.read_csv(os.path.join(CSV_DIR, name))

warnings.filterwarnings("ignore", category=FutureWarning)
sns.set_theme(style="whitegrid")

snaps    = r("snapshots.csv")    # time_s,server_id,connections,owd_ms
arrivals = r("arrivals.csv")     # time_s,session_id
req      = r("requests.csv")     # server_id,session_id,start_s,end_s,duration
drops    = r("drops.csv")        # server_id,session_id,time_s,reason
cfg      = r("servers.csv")      # id,mbps,owd_ms,max_conn
summ     = r("summary.csv")      # id,picked,served,dropped

n_srv   = req.server_id.nunique()
palette = sns.color_palette("tab20", n_colors=n_srv)

# ==== вспомогательные функции ===============================================

def adjust_xticks(ax, total):
    ax.set_xticklabels(ax.get_xticklabels(), rotation=90, ha="center")
    if total > 60:
        for label in ax.get_xticklabels()[::2]:
            label.set_visible(False)

# ==== 1. Прибытия + активные сессии ==========================================
fig1, (ax_a, ax_s) = plt.subplots(2, 1, figsize=(30, 20))
step = 1.0
arrivals["bin"] = (arrivals.time_s // step) * step
ts = arrivals.groupby("bin").size().reset_index(name="count")
sns.lineplot(data=ts, x="bin", y="count", ax=ax_a, marker="o", linewidth=1)
ax_a.set(title=f"Прибытия / {step:.0f}с", xlabel="время (с)", ylabel="прибытия")

sess = snaps.groupby("time_s").connections.sum().reset_index()
sns.lineplot(data=sess, x="time_s", y="connections", ax=ax_s, linewidth=1.2)
ax_s.set(title="Активные сессии", xlabel="время (с)", ylabel="сессии")

fig1.tight_layout()
fig1.savefig(os.path.join(PLOT_DIR, "arrivals_sessions.png"))
plt.close(fig1)

# ==== 2. RTT: рассеяние и распределение =======================================
fig2, (ax_sc, ax_box) = plt.subplots(2, 1, figsize=(30, 20))

sns.scatterplot(
    data=req, x="start_s", y="duration",
    hue="server_id", palette=palette,
    s=8, linewidth=0, legend=False, ax=ax_sc)
ax_sc.set(title="RTT во времени", xlabel="время (с)", ylabel="RTT (с)")

sns.violinplot(data=req, x="server_id", y="duration", palette=palette, inner=None, alpha=.3, ax=ax_box)
sns.boxplot(data=req, x="server_id", y="duration", palette=palette, fliersize=2, width=.6, ax=ax_box)
adjust_xticks(ax_box, n_srv)
ax_box.set(title="RTT по серверам", xlabel="сервер", ylabel="RTT (с)")

fig2.tight_layout()
fig2.savefig(os.path.join(PLOT_DIR, "rtt.png"))
plt.close(fig2)

# ==== 3. Одновременные соединения на сервер ===================================
fig_w = max(20, n_srv * 0.18)
plt.figure(figsize=(fig_w, 10))
ax_cb = sns.boxplot(data=snaps, x="server_id", y="connections", palette=palette, fliersize=2, width=.6)
sns.stripplot(data=snaps, x="server_id", y="connections", color="black", alpha=.15, size=1, ax=ax_cb)
adjust_xticks(ax_cb, n_srv)
ax_cb.set(title="Одновременные соединения по серверам", xlabel="сервер", ylabel="соединения")
plt.tight_layout()
plt.savefig(os.path.join(PLOT_DIR, "conn_box.png"))
plt.close()

# ==== 4. Тепловая карта соединений ============================================
bin_h = 1.0
snaps["bin"] = (snaps.time_s // bin_h) * bin_h
heat = snaps.groupby(["server_id", "bin"]).connections.max().unstack(fill_value=0)
fig_w = max(8, heat.shape[1] * 0.05)
plt.figure(figsize=(fig_w, 10))
sns.heatmap(heat, cmap="rocket_r", cbar_kws={"label": "соединения"}, yticklabels=True)
plt.title(f"Одновременные передачи / корзина {bin_h:.0f}с")
plt.xlabel("время (с)")
plt.ylabel("сервер")
plt.tight_layout()
plt.savefig(os.path.join(PLOT_DIR, "heatmap_connections.png"))
plt.close()

# ==== 4.1  Тепловая карта использования (активн./max_conn) =====================
max_conn = cfg.set_index("id")["max_conn"]
snaps["util"] = snaps.connections / snaps.server_id.map(max_conn)

heat_u = snaps.groupby(["server_id", "bin"]).util.max().unstack(fill_value=0)
fig_w_u = max(8, heat_u.shape[1] * 0.05)
plt.figure(figsize=(fig_w_u, 10))
sns.heatmap(heat_u, cmap="rocket_r", vmin=0, vmax=1, cbar_kws={"label": "загрузка"}, yticklabels=True)
plt.title(f"Использование (соединения / max_conn) / корзина {bin_h:.0f}с")
plt.xlabel("время (с)")
plt.ylabel("сервер")
plt.tight_layout()
plt.savefig(os.path.join(PLOT_DIR, "heatmap_utilisation.png"))
plt.close()

# ==== 5. Потери и бар‑плоты ====================================================
if not drops.empty:
    fig3, (ax1, ax2) = plt.subplots(2, 1, figsize=(30, 20))
    drops["bin"] = (drops.time_s // 1.0) * 1.0
    mix = drops.groupby("bin").size().reset_index(name="drops")
    ax1.bar(mix["bin"], mix["drops"], color="tab:red", width=0.8)
    ax1.set(title="Потери по корзинам", ylabel="потери", xlabel="время (с)")
    xt, n_bins = mix["bin"].values, len(mix)
    ax1.set_xticks(xt[::max(1, n_bins // 20)])
    ax1.set_xticklabels([int(x) for x in xt[::max(1, n_bins // 20)]], rotation=90, ha="center")

    bar = drops.groupby("server_id").size().reset_index(name="drops")
    sns.barplot(data=bar, x="server_id", y="drops", palette=palette, ax=ax2)
    adjust_xticks(ax2, n_srv)
    ax2.set(title="Потери в сумме по серверам", xlabel="сервер", ylabel="потери")

    fig3.tight_layout()
    fig3.savefig(os.path.join(PLOT_DIR, "drops.png"))
    plt.close(fig3)

    # ---- 5.1 Тепловая карта потерь ------------------------------------------
    heat_d = drops.groupby(["server_id", "bin"]).size().unstack(fill_value=0)
    fig_w = max(8, heat_d.shape[1] * 0.05)
    plt.figure(figsize=(fig_w, 10))
    sns.heatmap(heat_d, cmap="rocket_r", cbar_kws={"label": "потери"}, yticklabels=True)
    plt.title("Потери по серверам / корзина 1с")
    plt.xlabel("время (с)")
    plt.ylabel("сервер")
    plt.tight_layout()
    plt.savefig(os.path.join(PLOT_DIR, "heatmap_drops.png"))
    plt.close()

# ==== 6. Конфигурация + сводка ================================================
fig, (ax_t, ax_b, ax_c) = plt.subplots(3, 1, figsize=(30, 20))

# таблица конфигурации
_tbl = ax_t.table(cellText=cfg.values, colLabels=cfg.columns, loc="center", cellLoc="center")
_tbl.auto_set_font_size(False)
_tbl.set_fontsize(14)
_tbl.scale(1, 1.5)
ax_t.axis("off")
ax_t.set_title("Конфигурация серверов")

# обслужено / отклонено
m = summ.melt(id_vars="id", value_vars=["served", "dropped"], var_name="metric", value_name="count")
metric_map = {"served": "обслужено", "dropped": "отклонено"}
ax_b = sns.barplot(data=m, x="id", y="count", hue="metric", palette={"served": "tab:green", "dropped": "tab:red"}, ax=ax_b)
handles, labels = ax_b.get_legend_handles_labels()
ax_b.legend(handles, [metric_map.get(l, l) for l in labels])
adjust_xticks(ax_b, n_srv)
ax_b.set(title="Результаты симуляции", xlabel="сервер", ylabel="количество")

# назначено (picked)
mp = summ.melt(id_vars="id", value_vars=["picked"], var_name="metric", value_name="count")
ax_c = sns.barplot(data=mp, x="id", y="count", hue="metric", palette={"picked": "tab:blue"}, ax=ax_c)
ax_c.get_legend().remove()
adjust_xticks(ax_c, n_srv)
ax_c.set(title="Назначенные запросы", xlabel="сервер", ylabel="количество")

fig.tight_layout()
fig.savefig(os.path.join(PLOT_DIR, "config_results.png"))
plt.close(fig)

# ==== 7. Динамика и распределение OWD =========================================
owd = snaps.rename(columns={"owd_ms": "OWD"})
if n_srv <= 10:
    plt.figure(figsize=(10, 4))
    sns.lineplot(data=owd, x="time_s", y="OWD", hue="server_id", palette=palette, linewidth=1)
    plt.title("Динамика OWD по серверам")
    plt.xlabel("время (с)"); plt.ylabel("OWD (мс)")
    plt.tight_layout()
    plt.savefig(os.path.join(PLOT_DIR, "owd_multi.png"))
    plt.close()
else:
    cols = 5
    g = sns.FacetGrid(owd, col="server_id", col_wrap=cols, sharey=True, height=2, aspect=1.4)
    g.map_dataframe(sns.lineplot, x="time_s", y="OWD", linewidth=0.8)
    g.set_titles("серв {col_name}")
    g.set_axis_labels("время (с)", "OWD (мс)")
    for ax in g.axes.flatten():
        ax.tick_params(labelbottom=True, labelleft=True, rotation=0)
    plt.subplots_adjust(top=0.90)
    g.fig.suptitle("Динамика OWD по серверам")
    g.savefig(os.path.join(PLOT_DIR, "owd_grid.png"))
    plt.close(g.fig)

# распределения OWD
bins, cols = 40, 5
g = sns.FacetGrid(snaps, col="server_id", col_wrap=cols, sharex=False, sharey=False, height=2.4, aspect=1.3)
g.map_dataframe(sns.histplot, x="owd_ms", bins=bins, kde=True, linewidth=0)
g.set_titles("серв {col_name}")
g.set_axis_labels("OWD (мс)", "количество")
for ax in g.axes.flatten():
    ax.grid(True, linestyle=":", linewidth=0.5, alpha=0.6)
plt.subplots_adjust(top=0.88)
g.fig.suptitle("Распределение односторонней задержки (OWD) по серверам")
g.savefig(os.path.join(PLOT_DIR, "owd_dist.png"))
plt.close(g.fig)

# ==== 8. Глобальное распределение RTT =========================================
plt.figure(figsize=(12, 6))
req["rtt_ms"] = req.duration * 1_000
bins = 120
sns.histplot(data=req, x="rtt_ms", bins=bins, kde=True, kde_kws={"bw_adjust": .7}, color="tab:blue", edgecolor=None)
plt.title("Распределение RTT для всех запросов")
plt.xlabel("RTT (мс)")
plt.ylabel("запросы")

# логарифмическая ось X при длинном хвосте
if req.rtt_ms.max() / req.rtt_ms.min() > 100:
    plt.xscale("log")
    plt.xlabel("RTT (мс) – лог шкала")

plt.tight_layout()
plt.savefig(os.path.join(PLOT_DIR, "rtt_distribution.png"))
plt.close()

# ==== 9. Справедливость и CV во времени (загрузка кластера) ====================
util = snaps.copy()
util["u"] = util.connections / cfg.set_index("id").loc[util.server_id, "max_conn"].values

step_s = 1.0
util["bin"] = (util.time_s // step_s) * step_s
agg = util.groupby("bin").u.agg(["mean", "std", "sum", "count"])
agg["jain"] = agg["sum"]**2 / (agg["count"] * (util.groupby("bin").u.apply(lambda x: (x**2).sum())))
agg["cv"]   = agg["std"] / agg["mean"]

fig, ax1 = plt.subplots(figsize=(14, 6))
ax1.plot(agg.index, agg.jain, label="Индекс Джейна", lw=2)
ax1.set_ylabel("J(t)")
ax1.set_ylim(0, 1)
ax1.set_xlabel("время (с)")

ax2 = ax1.twinx()
ax2.plot(agg.index, agg.cv, label="CV нагрузки", color="tab:red", lw=1.2)
ax2.set_ylabel("CV(t)")

ax1.legend(loc="upper left")
ax2.legend(loc="upper right")
plt.title("Баланс нагрузки кластера во времени")
plt.tight_layout()
plt.savefig(os.path.join(PLOT_DIR, "fairness_cv.png"))
plt.close()

print(f"PNG-файлы сохранены в {PLOT_DIR}")
