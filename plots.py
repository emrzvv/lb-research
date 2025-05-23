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
sim_time = 600.0
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

# 2. RTT scatter + box/violin
fig2, (ax_sc, ax_box) = plt.subplots(2, 1, figsize=(30, 20))

# scatter
sns.scatterplot(data=req, x="start_s", y="duration",
                hue="server_id", palette=palette,
                s=8, linewidth=0, legend=False, ax=ax_sc)
ax_sc.set(title="RTT vs time", xlabel="time (s)", ylabel="RTT (s)")

# box + violin
sns.violinplot(data=req, x="server_id", y="duration",
               palette=palette, inner=None, alpha=.3, ax=ax_box)
sns.boxplot(data=req, x="server_id", y="duration",
            palette=palette, fliersize=2, width=.6, ax=ax_box)
adjust_xticks(ax_box, n_srv)
ax_box.set(title="RTT per server", xlabel="server", ylabel="RTT (s)")

fig2.tight_layout()
fig2.savefig("plots/rtt.png")
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
plt.figure(figsize=(fig_w, 10))
sns.heatmap(heat, cmap="rocket_r", cbar_kws={"label": "connections"}, yticklabels=True)
plt.title(f"Concurrent transfers / bin {bin_h:.0f}s")
plt.xlabel("time (s)"); plt.ylabel("server")
plt.tight_layout(); plt.savefig("plots/heatmap_connections.png"); plt.close()


# 5. DROPS 
drops_step = 1.0
if drops is not None and not drops.empty:
    fig3, (ax1, ax2) = plt.subplots(2, 1, figsize=(30, 20))

    drops["bin"] = (drops.time_s // drops_step) * drops_step
    mix = drops.groupby("bin").size().reset_index(name="drops")

    # barplot по времени
    bars = ax1.bar(mix["bin"], mix["drops"], color="tab:red", width=0.8)
    ax1.set(title="Drops per bin", ylabel="drops", xlabel="time (s)")

    # Сделаем xticks разреженными (как в heatmap)
    n_bins = len(mix["bin"])
    xticks = mix["bin"].values
    # показываем не больше 20 подписей
    tick_step = max(1, n_bins // 20)
    shown_xticks = xticks[::tick_step]
    ax1.set_xticks(shown_xticks)
    ax1.set_xticklabels([f"{int(x):d}" for x in shown_xticks], rotation=90, ha='center')

    # barplot по серверу
    bar = drops.groupby("server_id").size().reset_index(name="drops")
    sns.barplot(data=bar, x="server_id", y="drops",
                palette=palette, ax=ax2)
    adjust_xticks(ax2, n_srv)
    ax2.set(title="Total drops per server", xlabel="server", ylabel="drops")

    fig3.tight_layout(); fig3.savefig("plots/drops.png"); plt.close(fig3)

# 5.1 HEATMAP DROPS
drops_step = 1.0
if drops is not None and not drops.empty:
    drops["bin"] = (drops.time_s // drops_step) * drops_step
    heat_drops = drops.groupby(["server_id", "bin"]).size().unstack(fill_value=0)
    fig_w = max(8, heat_drops.shape[1] * 0.05)
    plt.figure(figsize=(fig_w, 10))
    sns.heatmap(heat_drops, cmap="rocket_r", cbar_kws={"label": "drops"}, yticklabels=True)
    plt.title(f"Drops per server / bin {drops_step:.0f}s")
    plt.xlabel("time (s)")
    plt.ylabel("server")
    plt.tight_layout()
    plt.savefig("plots/heatmap_drops.png")
    plt.close()

# 6. CONFIG + SUMMARY

fig, (ax_t, ax_b, ax_c) = plt.subplots(3, 1, figsize=(30, 20))

# таблица конфигурации
tbl = ax_t.table(
    cellText=cfg.values,
    colLabels=cfg.columns,
    loc="center", cellLoc="center")
tbl.auto_set_font_size(False)
tbl.set_fontsize(14)
tbl.scale(1, 1.5)
ax_t.axis("off")
ax_t.set_title("Server configuration")

# bar-chart итогов
m = summ.melt(id_vars="id", value_vars=["served","dropped"],
              var_name="metric", value_name="count")
sns.barplot(data=m, x="id", y="count", hue="metric",
            palette={"served":"tab:green", "dropped":"tab:red"},
            ax=ax_b)
adjust_xticks(ax_b, n_srv)
ax_b.set(title="Simulation results", xlabel="server", ylabel="count")
ax_b.legend(title="", loc="upper right")

mp = summ.melt(id_vars="id", value_vars=["picked"],
               var_name="metric", value_name="count")
sns.barplot(data=mp, x="id", y="count", hue="metric",
            palette={"picked":"tab:blue"},
            ax=ax_c)
adjust_xticks(ax_c, n_srv)
ax_c.set(title="Simulation results", xlabel="server", ylabel="count")
ax_c.legend(title="", loc="upper right")

fig.tight_layout()
fig.savefig("plots/config_results.png")
plt.close(fig)

# 8. Динамика OWD (из snapshots)
owd = snaps.rename(columns={"owd_ms": "OWD"})   # time_s, server_id, OWD
if n_srv <= 10:
    # один график, разные цвета
    plt.figure(figsize=(10, 4))
    sns.lineplot(data=owd, x="time_s", y="OWD",
                 hue="server_id", palette=palette, linewidth=1)
    plt.title("OWD dynamics per server")
    plt.xlabel("time (s)"); plt.ylabel("OWD (ms)")
    plt.tight_layout(); plt.savefig("plots/owd_multi.png"); plt.close()
else:
    # решётка 5×k, общий масштаб по Y
    cols = 5
    g = sns.FacetGrid(owd, col="server_id", col_wrap=cols,
                      sharey=True, height=2, aspect=1.4)
    g.map_dataframe(sns.lineplot, x="time_s", y="OWD", linewidth=0.8)
    g.set_titles("srv {col_name}")
    g.set_axis_labels("time (s)", "OWD (ms)")
    for ax in g.axes.flatten():
        ax.tick_params(labelbottom=True, labelleft=True, rotation=0)
    plt.subplots_adjust(top=0.90)
    g.fig.suptitle("OWD dynamics per server")
    g.savefig("plots/owd_grid.png")
    plt.close(g.fig)

# 8.1 DISTRIBUTION OF OWD PER SERVER
# один facet-grid: по столбцам server_id, внутри — гистограмма + KDE
bins = 40                     # мелкие столбики, чтобы был «хвост»
cols = 5                      # сколько графиков в строке
g = sns.FacetGrid(
        snaps,
        col="server_id",
        col_wrap=cols,
        sharex=False,  # задержки разных серверов могут отличаться по масштабам
        sharey=False,
        height=2.4,
        aspect=1.3,
)

g.map_dataframe(sns.histplot,
                x="owd_ms",
                bins=bins,
                kde=True,      # поверх гистограммы — сглаженная плотность
                linewidth=0)

g.set_titles("srv {col_name}")
g.set_axis_labels("OWD (ms)", "count")
for ax in g.axes.flatten():
    ax.grid(True, linestyle=":", linewidth=0.5, alpha=0.6)

plt.subplots_adjust(top=0.88)
g.fig.suptitle("Distribution of one-way delay (OWD) per server")
g.savefig("plots/owd_dist.png")
plt.close(g.fig)

print("PNG-файлы сохранены в ./plots")
