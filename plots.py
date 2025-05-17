#!/usr/bin/env python3

import os, math, warnings
import pandas as pd
import seaborn as sns
import matplotlib.pyplot as plt

warnings.filterwarnings('ignore', category=FutureWarning)
sns.set_theme(style="whitegrid")
os.makedirs("plots", exist_ok=True)

snaps = pd.read_csv("csv/snapshots.csv")   # time_s,server_id,connections,owd_ms
arrivals = pd.read_csv("csv/arrivals.csv")    # time_s
req = pd.read_csv("csv/requests.csv")    # server_id,start_s,end_s,duration
drops = pd.read_csv("csv/drops.csv") # server_id,time_s,reason
cfg  = pd.read_csv("csv/servers.csv")   # id,mbps,owd_ms,max_conn
summ = pd.read_csv("csv/summary.csv")   # id,picked,served,dropped

n_srv = req.server_id.nunique()
palette = sns.color_palette("tab20", n_colors=n_srv)

#поворот и разрежение xticks
def adjust_xticks(ax, total, step=1):
    ax.set_xticklabels(ax.get_xticklabels(), rotation=90, ha='center')
    if total > 60:  # оставляем каждую 2-ю подпись
        for label in ax.get_xticklabels()[::2]:
            label.set_visible(False)

# 1. ARRIVALS + ACTIVE SESSIONS 
fig1, (ax_a, ax_s) = plt.subplots(2, 1, figsize=(30, 20), sharex=False)

# arrivals per step
step = 1.0
arrivals["bin"] = (arrivals.time_s // step) * step
ts = arrivals.groupby("bin").size().reset_index(name="count")
sns.lineplot(data=ts, x="bin", y="count", ax=ax_a, marker="o", linewidth=1)
ax_a.set(title=f"Arrivals / {step:.0f}s", xlabel="time (s)", ylabel="arrivals")

# active sessions
sess = snaps.groupby("time_s").connections.sum().reset_index()
sns.lineplot(data=sess, x="time_s", y="connections", ax=ax_s, linewidth=1.2)
ax_s.set(title="Active sessions", xlabel="time (s)", ylabel="sessions")

fig1.tight_layout()
fig1.savefig("plots/arrivals_sessions.png")
plt.close(fig1)

# 2. LATENCY scatter + box/violin
fig2, (ax_sc, ax_box) = plt.subplots(2, 1, figsize=(30, 20))

# scatter
sns.scatterplot(data=req, x="start_s", y="duration",
                hue="server_id", palette=palette,
                s=8, linewidth=0, legend=False, ax=ax_sc)
ax_sc.set(title="Latency vs time", xlabel="time (s)", ylabel="latency (s)")

# box + violin
sns.violinplot(data=req, x="server_id", y="duration",
               palette=palette, inner=None, alpha=.3, ax=ax_box)
sns.boxplot(data=req, x="server_id", y="duration",
            palette=palette, fliersize=2, width=.6, ax=ax_box)
adjust_xticks(ax_box, n_srv)
ax_box.set(title="Latency per server", xlabel="server", ylabel="latency (s)")

fig2.tight_layout()
fig2.savefig("plots/latency.png")
plt.close(fig2)

# 3. CONN_BOX 
fig_w = max(20, n_srv * 0.18)
plt.figure(figsize=(fig_w, 10))
ax_cb = sns.boxplot(data=snaps, x="server_id", y="connections",
                    palette=palette, fliersize=2, width=.6)
sns.stripplot(data=snaps, x="server_id", y="connections",
              color="black", alpha=.15, size=1, ax=ax_cb)
adjust_xticks(ax_cb, n_srv)
ax_cb.set(title="Concurrent connections per server",
          xlabel="server", ylabel="connections")
plt.tight_layout(); plt.savefig("plots/conn_box.png"); plt.close()

# 4. HEATMAP
bin_h = 1.0
snaps["bin"] = (snaps.time_s // bin_h) * bin_h
heat = snaps.groupby(["server_id", "bin"]).connections.max().unstack(fill_value=0)
fig_w = max(8, heat.shape[1] * 0.05)
plt.figure(figsize=(fig_w, 8))
sns.heatmap(heat, cmap="rocket_r", cbar_kws={"label": "conns"}, yticklabels=True)
plt.title(f"Concurrent transfers • bin {bin_h:.0f}s")
plt.xlabel("time (s)"); plt.ylabel("server")
plt.tight_layout(); plt.savefig("plots/heatmap.png"); plt.close()

# 5. DROPS 
if drops is not None and not drops.empty:
    fig3, (ax1, ax2) = plt.subplots(2, 1, figsize=(12, 6))

    drops["bin"] = (drops.time_s // step) * step
    mix = arrivals.groupby("bin").size().reset_index(name="arr") \
           .merge(drops.groupby("bin").size()
                  .reset_index(name="drops"), on="bin", how="left").fillna(0)
    mix["pct"] = 100 * mix.drops / mix.arr

    sns.barplot(data=mix, x="bin", y="drops", color="tab:red", ax=ax1)
    ax1b = ax1.twinx()
    sns.lineplot(data=mix, x="bin", y="pct", ax=ax1b, color="tab:gray")
    ax1.set(title="Drop rate", ylabel="drops"); ax1b.set_ylabel("% arrivals")

    bar = drops.groupby("server_id").size().reset_index(name="drops")
    sns.barplot(data=bar, x="server_id", y="drops",
                palette=palette, ax=ax2)
    adjust_xticks(ax2, n_srv)
    ax2.set(title="Total drops per server", xlabel="server", ylabel="drops")

    fig3.tight_layout(); fig3.savefig("plots/drops.png"); plt.close(fig3)

# 6. CONFIG + SUMMARY

fig, (ax_t, ax_b) = plt.subplots(2, 1, figsize=(30, 20))

# таблица конфигурации
tbl = ax_t.table(
    cellText=cfg.values,
    colLabels=cfg.columns,
    loc="center", cellLoc="center")
tbl.auto_set_font_size(False)
tbl.set_fontsize(8)
tbl.scale(1, 1.5)
ax_t.axis("off")
ax_t.set_title("Server configuration")

# bar-chart итогов
m = summ.melt(id_vars="id", value_vars=["served","dropped","picked"],
              var_name="metric", value_name="count")
sns.barplot(data=m, x="id", y="count", hue="metric",
            palette={"served":"tab:blue", "dropped":"tab:red", "picked":"tab:gray"},
            ax=ax_b)
adjust_xticks(ax_b, n_srv)
ax_b.set(title="Simulation results", xlabel="server", ylabel="count")
ax_b.legend(title="", loc="upper right")

fig.tight_layout()
fig.savefig("plots/config_results.png")
plt.close(fig)

print("PNG-файлы сохранены в ./plots")
