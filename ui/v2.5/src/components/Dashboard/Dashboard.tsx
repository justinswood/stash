import React, { useState, useRef, useMemo, useEffect } from "react";
import { useHistory } from "react-router-dom";
import TextUtils from "src/utils/text";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { getPlatformURL } from "src/core/createClient";

// ---- Dashboard data types ----
interface DStats {
  scene_count: number;
  image_count: number;
  gallery_count: number;
  performer_count: number;
  studio_count: number;
  group_count: number;
  tag_count: number;
  total_o_count: number;
  total_play_count: number;
  scenes_played: number;
  scenes_duration: number;
  total_play_duration: number;
  scenes_size: number;
  images_size: number;
}
interface DStudio { id: string; name: string; scene_count: number; }
interface DPerformer { id: string; name: string; scene_count: number; }
interface DTag { id: string; name: string; scene_count: number; }
interface DAddedScene { id: string; created_at: string; }
interface DPlayedScene { id: string; last_played_at: string | null; play_duration: number | null; }
interface DHealthCounts {
  noFiles: number; duplicates: number; lowRes: number;
  noPerformers: number; untagged: number; missingPhash: number;
}
interface DashboardData {
  ready: boolean;
  stats: DStats | null;
  studios: DStudio[];
  performers: DPerformer[];
  tags: DTag[];
  addedScenes: DAddedScene[];
  playedScenes: DPlayedScene[];
  health: DHealthCounts;
}

const INITIAL_DATA: DashboardData = {
  ready: false,
  stats: null, studios: [], performers: [], tags: [],
  addedScenes: [], playedScenes: [],
  health: { noFiles: 0, duplicates: 0, lowRes: 0, noPerformers: 0, untagged: 0, missingPhash: 0 },
};

// Single hook fetches all dashboard data with 3 parallel raw-fetch calls (no Apollo).
function useDashboardData(): DashboardData {
  const [data, setData] = useState<DashboardData>(INITIAL_DATA);
  useEffect(() => {
    const url = getPlatformURL("graphql").toString();
    const post = (query: string) =>
      fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ query }),
      }).then((r) => r.json());

    const since = daysAgoISO(90);
    const n = (x: unknown): number => (typeof x === "number" ? x : 0);

    Promise.all([
      // Request 1: stats + top studios + top performers + top tags
      post(`{
        stats { scene_count image_count gallery_count performer_count studio_count group_count tag_count total_o_count total_play_count scenes_played scenes_duration total_play_duration scenes_size images_size }
        findStudios(filter:{per_page:5,sort:"scenes_count",direction:DESC}) { studios { id name scene_count } }
        findPerformers(filter:{per_page:8,sort:"scenes_count",direction:DESC}) { performers { id name scene_count } }
        findTags(filter:{per_page:50,sort:"scenes_count",direction:DESC}) { tags { id name scene_count } }
      }`),
      // Request 2: activity — added + played scenes in last 90 days (aliased)
      post(`{
        addedScenes: findScenes(filter:{per_page:2000,sort:"created_at",direction:DESC},scene_filter:{created_at:{modifier:GREATER_THAN,value:"${since}"}}) { scenes { id created_at } }
        playedScenes: findScenes(filter:{per_page:2000,sort:"last_played_at",direction:DESC},scene_filter:{last_played_at:{modifier:GREATER_THAN,value:"${since}"}}) { scenes { id last_played_at play_duration } }
      }`),
      // Request 3: library health counts (aliased into a single query)
      post(`{
        noFiles: findScenes(filter:{per_page:0},scene_filter:{file_count:{modifier:EQUALS,value:0}}) { count }
        duplicates: findScenes(filter:{per_page:0},scene_filter:{duplicated:{duplicated:true}}) { count }
        lowRes: findScenes(filter:{per_page:0},scene_filter:{resolution:{modifier:LESS_THAN,value:STANDARD_HD}}) { count }
        noPerformers: findScenes(filter:{per_page:0},scene_filter:{performer_count:{modifier:EQUALS,value:0}}) { count }
        untagged: findScenes(filter:{per_page:0},scene_filter:{tag_count:{modifier:EQUALS,value:0}}) { count }
        missingPhash: findScenes(filter:{per_page:0},scene_filter:{is_missing:"phash"}) { count }
      }`),
    ])
      .then(([main, activity, health]) => {
        setData({
          ready: true,
          stats: main.data?.stats ?? null,
          studios: main.data?.findStudios?.studios ?? [],
          performers: main.data?.findPerformers?.performers ?? [],
          tags: main.data?.findTags?.tags ?? [],
          addedScenes: activity.data?.addedScenes?.scenes ?? [],
          playedScenes: activity.data?.playedScenes?.scenes ?? [],
          health: {
            noFiles:      n(health.data?.noFiles?.count),
            duplicates:   n(health.data?.duplicates?.count),
            lowRes:       n(health.data?.lowRes?.count),
            noPerformers: n(health.data?.noPerformers?.count),
            untagged:     n(health.data?.untagged?.count),
            missingPhash: n(health.data?.missingPhash?.count),
          },
        });
      })
      .catch(() => {});
  }, []);
  return data;
}

// ---- Helpers ----

function fmtNum(n: number): string {
  return n.toLocaleString("en-US");
}

function clamp(x: number, a: number, b: number): number {
  return Math.max(a, Math.min(b, x));
}

function fmtSize(bytes: number): { value: string; unit: string } {
  const tib = bytes / Math.pow(1024, 4);
  if (tib >= 0.1) return { value: tib.toFixed(1), unit: "TiB" };
  const gib = bytes / Math.pow(1024, 3);
  if (gib >= 0.1) return { value: gib.toFixed(1), unit: "GiB" };
  const mib = bytes / Math.pow(1024, 2);
  return { value: mib.toFixed(1), unit: "MiB" };
}

function bucketByDay(dates: string[], days: number): number[] {
  const now = Date.now();
  const result = new Array(days).fill(0);
  for (const d of dates) {
    if (!d) continue;
    const daysAgo = Math.floor((now - new Date(d).getTime()) / 86400000);
    if (daysAgo >= 0 && daysAgo < days) {
      result[days - 1 - daysAgo]++;
    }
  }
  return result;
}

function bucketDurationByDay(
  items: Array<{ date: string | null; duration: number | null }>,
  days: number
): number[] {
  const now = Date.now();
  const result = new Array(days).fill(0);
  for (const item of items) {
    if (!item.date) continue;
    const daysAgo = Math.floor(
      (now - new Date(item.date).getTime()) / 86400000
    );
    if (daysAgo >= 0 && daysAgo < days) {
      result[days - 1 - daysAgo] += item.duration ?? 0;
    }
  }
  return result;
}

function daysAgoISO(n: number): string {
  return new Date(Date.now() - n * 86400000).toISOString();
}

// ---- Palette ----
const STUDIO_PALETTE = [
  "#5ab9c9",
  "#c98b5a",
  "#8ec95a",
  "#c95a8e",
  "#9a5ac9",
  "#3a4250",
];

// ---- Sparkline ----
interface SparklineProps {
  data: number[];
  width?: number;
  height?: number;
}
function Sparkline({ data, width = 64, height = 18 }: SparklineProps) {
  if (!data.length) return null;
  const max = Math.max(...data, 1);
  const pts = data.map((v, i) => {
    const x = (i / Math.max(data.length - 1, 1)) * width;
    const y = height - (v / max) * (height - 2) - 1;
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  });
  const path = "M" + pts.join(" L");
  const area = `${path} L${width},${height} L0,${height} Z`;
  return (
    <svg
      width={width}
      height={height}
      className="db-spark"
      style={{ display: "block" }}
    >
      <path d={area} fill="#5ab9c9" fillOpacity="0.12" />
      <path d={path} fill="none" stroke="#5ab9c9" strokeWidth="1.25" />
    </svg>
  );
}

// ---- StatCard ----
interface StatCardProps {
  label: string;
  value: string | number;
  unit?: string;
  sub?: string;
  spark?: number[];
  onClick?: () => void;
}
function StatCard({ label, value, unit, sub, spark, onClick }: StatCardProps) {
  return (
    <button
      className="db-stat"
      onClick={onClick}
      type="button"
      title={label}
    >
      <div className="db-stat__head">
        <span className="db-stat__label">{label}</span>
      </div>
      <div className="db-stat__value">
        <span className="db-stat__num">{value}</span>
        {unit && <span className="db-stat__unit">{unit}</span>}
      </div>
      {(sub || spark) && (
        <div className="db-stat__foot">
          {sub && <span className="db-stat__sub">{sub}</span>}
          {spark && <Sparkline data={spark} />}
        </div>
      )}
    </button>
  );
}

// ---- StatGrid ----
interface StatGridProps {
  stats: DStats | null;
  sparkAdded: number[];
  sparkPlays: number[];
}
function StatGrid({ stats, sparkAdded, sparkPlays }: StatGridProps) {
  const history = useHistory();

  if (!stats) {
    return (
      <div className="db-statgrid">
        {Array.from({ length: 12 }).map((_, i) => (
          <div key={i} className="db-stat db-stat--skeleton" />
        ))}
      </div>
    );
  }

  const s = stats;
  const scenesSize = fmtSize(s.scenes_size);
  const imagesSize = fmtSize(s.images_size);
  const scenesDur = TextUtils.secondsAsTimeString(s.scenes_duration, 3);
  const playDur = TextUtils.secondsAsTimeString(s.total_play_duration, 3);

  const cards: StatCardProps[] = [
    {
      label: "SCENES",
      value: fmtNum(s.scene_count),
      spark: sparkAdded,
      onClick: () => history.push("/scenes"),
    },
    {
      label: "SCENES SIZE",
      value: scenesSize.value,
      unit: scenesSize.unit,
      onClick: () => history.push("/scenes?disp=1"),
    },
    {
      label: "DURATION",
      value: scenesDur || "—",
      onClick: () => history.push("/scenes"),
    },
    {
      label: "PERFORMERS",
      value: fmtNum(s.performer_count),
      onClick: () => history.push("/performers"),
    },
    {
      label: "STUDIOS",
      value: fmtNum(s.studio_count),
      onClick: () => history.push("/studios"),
    },
    {
      label: "TAGS",
      value: fmtNum(s.tag_count),
      onClick: () => history.push("/tags"),
    },
    {
      label: "IMAGES",
      value: fmtNum(s.image_count),
      sub: `${imagesSize.value} ${imagesSize.unit}`,
      onClick: () => history.push("/images"),
    },
    {
      label: "GALLERIES",
      value: fmtNum(s.gallery_count),
      onClick: () => history.push("/galleries"),
    },
    {
      label: "GROUPS",
      value: fmtNum(s.group_count),
      onClick: () => history.push("/groups"),
    },
    {
      label: "PLAY COUNT",
      value: fmtNum(s.total_play_count),
      sub: `${fmtNum(s.scenes_played)} unique`,
      spark: sparkPlays,
      onClick: () => history.push("/scenes"),
    },
    {
      label: "PLAY DURATION",
      value: playDur || "—",
      onClick: () => history.push("/scenes"),
    },
    {
      label: "O-COUNTER",
      value: fmtNum(s.total_o_count),
      onClick: () => history.push("/scenes"),
    },
  ];

  return (
    <div className="db-statgrid">
      {cards.map((c) => (
        <StatCard key={c.label} {...c} />
      ))}
    </div>
  );
}

// ---- DashboardCard ----
interface DashboardCardProps {
  title: string;
  right?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}
function DashboardCard({
  title,
  right,
  children,
  className = "",
}: DashboardCardProps) {
  return (
    <section className={`db-card ${className}`}>
      <header className="db-card__head">
        <h3 className="db-card__title">{title}</h3>
        {right}
      </header>
      <div className="db-card__body">{children}</div>
    </section>
  );
}

// ---- ActivityChart ----
type ActivityMetric = "plays" | "added" | "duration";
type ActivityRange = 7 | 30 | 90;

interface ActivityChartProps {
  addedScenes: DAddedScene[];
  playedScenes: DPlayedScene[];
}
function ActivityChart({ addedScenes, playedScenes }: ActivityChartProps) {
  const [range, setRange] = useState<ActivityRange>(30);
  const [metric, setMetric] = useState<ActivityMetric>("added");
  const [hoverIdx, setHoverIdx] = useState<number | null>(null);
  const [hoverPos, setHoverPos] = useState({ x: 0, y: 0 });
  const chartRef = useRef<HTMLDivElement>(null);

  const { added90, plays90, duration90 } = useMemo(() => {
    const addedDates = addedScenes.map((s) => s.created_at);
    const playedItems = playedScenes.map((s) => ({
      date: s.last_played_at,
      duration: s.play_duration,
    }));
    const a90 = bucketByDay(addedDates, 90);
    const p90 = bucketByDay(
      playedItems.map((i) => i.date ?? ""),
      90
    );
    const d90 = bucketDurationByDay(playedItems, 90);
    return { added90: a90, plays90: p90, duration90: d90 };
  }, [addedScenes, playedScenes]);

  const datasets: Record<ActivityMetric, number[]> = {
    added: added90,
    plays: plays90,
    duration: duration90,
  };

  const metricLabel: Record<ActivityMetric, string> = {
    added: "scenes added",
    plays: "scenes played",
    duration: "seconds watched",
  };

  const fullSlice = datasets[metric];
  const slice = fullSlice.slice(-range);
  const prevSlice = fullSlice.slice(-range * 2, -range);
  const total = slice.reduce((a, b) => a + b, 0);
  const prevTotal = prevSlice.reduce((a, b) => a + b, 0);
  const delta =
    prevTotal === 0 ? 100 : Math.round(((total - prevTotal) / prevTotal) * 100);

  const maxVal = Math.max(...slice, 1);
  const W = 100;
  const H = 100;

  const points = slice.map((v, i) => {
    const x = (i / Math.max(slice.length - 1, 1)) * W;
    const y = H - (v / maxVal) * (H - 6) - 3;
    return [x, y];
  });
  const pathStr =
    "M" + points.map((p) => `${p[0].toFixed(2)},${p[1].toFixed(2)}`).join(" L");
  const areaStr = `${pathStr} L${W},${H} L0,${H} Z`;

  function onMouseMove(e: React.MouseEvent<HTMLDivElement>) {
    const rect = chartRef.current?.getBoundingClientRect();
    if (!rect) return;
    const x = (e.clientX - rect.left) / rect.width;
    const i = clamp(Math.round(x * (slice.length - 1)), 0, slice.length - 1);
    setHoverIdx(i);
    setHoverPos({ x: e.clientX - rect.left, y: e.clientY - rect.top });
  }

  const todayDate = new Date();
  const startDate = new Date(Date.now() - (range - 1) * 86400000);
  const midDate = new Date(Date.now() - Math.floor(range / 2) * 86400000);
  const fmtDate = (d: Date) =>
    d.toLocaleDateString("en-US", { month: "short", day: "numeric" });

  const displayValue =
    metric === "duration"
      ? Math.round(
          hoverIdx !== null ? slice[hoverIdx] / 60 : total / 60
        ) + "m"
      : fmtNum(hoverIdx !== null ? slice[hoverIdx] : total);

  return (
    <DashboardCard
      title="Activity"
      right={
        <div className="db-seg">
          {(["added", "plays", "duration"] as ActivityMetric[]).map((m) => (
            <button
              key={m}
              className={metric === m ? "on" : ""}
              onClick={() => setMetric(m)}
            >
              {m.charAt(0).toUpperCase() + m.slice(1)}
            </button>
          ))}
          <span className="db-seg__sep" />
          {([7, 30, 90] as ActivityRange[]).map((r) => (
            <button
              key={r}
              className={range === r ? "on" : ""}
              onClick={() => setRange(r)}
            >
              {r}d
            </button>
          ))}
        </div>
      }
    >
      <div className="db-activity">
        <div className="db-activity__head">
          <div>
            <div className="db-activity__total">{displayValue}</div>
            <div className="db-activity__label">
              {hoverIdx !== null
                ? fmtDate(new Date(Date.now() - (range - 1 - hoverIdx) * 86400000))
                : `total ${metricLabel[metric]} · last ${range}d`}
            </div>
          </div>
          {hoverIdx === null && (
            <div className={`db-activity__delta ${delta >= 0 ? "up" : "down"}`}>
              {delta >= 0 ? "▲" : "▼"} {Math.abs(delta)}%
              <span className="db-activity__delta-lbl">vs prior {range}d</span>
            </div>
          )}
        </div>
        <div
          className="db-chart"
          ref={chartRef}
          onMouseMove={onMouseMove}
          onMouseLeave={() => setHoverIdx(null)}
        >
          <svg
            viewBox={`0 0 ${W} ${H}`}
            preserveAspectRatio="none"
            className="db-chart__svg"
          >
            {[0.25, 0.5, 0.75].map((g) => (
              <line
                key={g}
                x1="0"
                x2={W}
                y1={H * g}
                y2={H * g}
                stroke="#222831"
                strokeWidth="0.25"
                strokeDasharray="1,1.5"
              />
            ))}
            <path d={areaStr} fill="#5ab9c9" fillOpacity="0.10" />
            <path
              d={pathStr}
              fill="none"
              stroke="#5ab9c9"
              strokeWidth="0.75"
              vectorEffect="non-scaling-stroke"
            />
            {hoverIdx !== null && points[hoverIdx] && (
              <>
                <line
                  x1={points[hoverIdx][0]}
                  x2={points[hoverIdx][0]}
                  y1="0"
                  y2={H}
                  stroke="#5ab9c9"
                  strokeWidth="0.4"
                  vectorEffect="non-scaling-stroke"
                />
                <circle
                  cx={points[hoverIdx][0]}
                  cy={points[hoverIdx][1]}
                  r="1.4"
                  fill="#0b0d10"
                  stroke="#5ab9c9"
                  strokeWidth="0.6"
                  vectorEffect="non-scaling-stroke"
                />
              </>
            )}
          </svg>
          {hoverIdx !== null && (
            <div
              className="db-chart__tip"
              style={{ left: hoverPos.x + 8, top: hoverPos.y - 8 }}
            >
              <div className="db-chart__tip-date">
                {fmtDate(
                  new Date(Date.now() - (range - 1 - hoverIdx) * 86400000)
                )}
              </div>
              <div className="db-chart__tip-val">
                {metric === "duration"
                  ? Math.round(slice[hoverIdx] / 60) + "m"
                  : fmtNum(slice[hoverIdx])}{" "}
                {metric}
              </div>
            </div>
          )}
        </div>
        <div className="db-chart__axis">
          <span>{fmtDate(startDate)}</span>
          <span>{fmtDate(midDate)}</span>
          <span>Today</span>
        </div>
      </div>
    </DashboardCard>
  );
}

// ---- TopPerformers ----
function TopPerformers({ performers, ready }: { performers: DPerformer[]; ready: boolean }) {
  const history = useHistory();
  const maxVal = Math.max(...performers.map((p) => p.scene_count), 1);

  return (
    <DashboardCard
      title="Top Performers"
      right={
        <div className="db-seg db-seg--sm">
          <button className="on">Scenes</button>
        </div>
      }
    >
      {!ready ? (
        <LoadingIndicator small />
      ) : (
        <ul className="db-leader">
          {performers.map((p, i) => {
            const initials = p.name
              .split(" ")
              .map((w) => w[0])
              .join("")
              .slice(0, 2)
              .toUpperCase();
            const hue = (i * 47) % 360;
            return (
              <li key={p.id} className="db-leader__row">
                <span className="db-leader__rank">
                  {String(i + 1).padStart(2, "0")}
                </span>
                <span
                  className="db-leader__avatar"
                  style={{ background: `hsl(${hue} 22% 28%)` }}
                >
                  {initials}
                </span>
                <div className="db-leader__main">
                  <a
                    className="db-leader__name"
                    href={`/performers/${p.id}`}
                    onClick={(e) => {
                      e.preventDefault();
                      history.push(`/performers/${p.id}`);
                    }}
                  >
                    {p.name}
                  </a>
                  <div className="db-leader__bar">
                    <div
                      className="db-leader__bar-fill"
                      style={{ width: `${(p.scene_count / maxVal) * 100}%` }}
                    />
                  </div>
                </div>
                <div className="db-leader__stat">
                  <div className="db-leader__num">{fmtNum(p.scene_count)}</div>
                  <div className="db-leader__trend-lbl">scenes</div>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </DashboardCard>
  );
}

// ---- TopStudios ----
function TopStudios({ studios, totalScenes, ready }: { studios: DStudio[]; totalScenes: number; ready: boolean }) {
  const [activeIdx, setActiveIdx] = useState<number | null>(null);

  if (!ready) {
    return (
      <DashboardCard title="Top Studios">
        <LoadingIndicator small />
      </DashboardCard>
    );
  }

  const top5Total = studios.reduce((a, s) => a + s.scene_count, 0);
  const otherCount = Math.max(0, totalScenes - top5Total);

  const allStudios = [
    ...studios.map((s, i) => ({
      id: s.id,
      name: s.name,
      scenes: s.scene_count,
      color: STUDIO_PALETTE[i],
    })),
    ...(otherCount > 0
      ? [{ id: "other", name: "Other", scenes: otherCount, color: STUDIO_PALETTE[5] }]
      : []),
  ];

  const donutTotal = allStudios.reduce((a, s) => a + s.scenes, 0);
  const r = 38;
  const c = 50;
  const circ = 2 * Math.PI * r;
  let acc = 0;

  const centerCount =
    activeIdx !== null && allStudios[activeIdx]
      ? allStudios[activeIdx].scenes
      : donutTotal;

  return (
    <DashboardCard title="Top Studios">
      <div className="db-studios">
        <div className="db-studios__donut">
          <svg viewBox="0 0 100 100">
            <circle
              cx={c}
              cy={c}
              r={r}
              fill="none"
              stroke="#1a1f27"
              strokeWidth="14"
            />
            {allStudios.map((s, i) => {
              const frac = donutTotal > 0 ? s.scenes / donutTotal : 0;
              const dash = frac * circ;
              const offset = -acc * circ;
              acc += frac;
              const isActive = activeIdx === i;
              return (
                <circle
                  key={s.id}
                  cx={c}
                  cy={c}
                  r={r}
                  fill="none"
                  stroke={s.color}
                  strokeWidth={isActive ? 16 : 14}
                  strokeDasharray={`${dash} ${circ - dash}`}
                  strokeDashoffset={offset}
                  transform={`rotate(-90 ${c} ${c})`}
                  style={{ transition: "stroke-width .15s", cursor: "pointer" }}
                  onMouseEnter={() => setActiveIdx(i)}
                  onMouseLeave={() => setActiveIdx(null)}
                />
              );
            })}
            <text
              x={c}
              y={c - 2}
              textAnchor="middle"
              className="db-studios__center-num"
            >
              {fmtNum(centerCount)}
            </text>
            <text
              x={c}
              y={c + 8}
              textAnchor="middle"
              className="db-studios__center-lbl"
            >
              scenes
            </text>
          </svg>
        </div>
        <ul className="db-studios__list">
          {allStudios.map((s, i) => {
            const pct =
              donutTotal > 0 ? ((s.scenes / donutTotal) * 100).toFixed(1) : "0.0";
            return (
              <li
                key={s.id}
                className={`db-studios__row ${activeIdx === i ? "is-active" : ""}`}
                onMouseEnter={() => setActiveIdx(i)}
                onMouseLeave={() => setActiveIdx(null)}
              >
                <span
                  className="db-studios__sw"
                  style={{ background: s.color }}
                />
                <span className="db-studios__name">{s.name}</span>
                <span className="db-studios__pct">{pct}%</span>
                <span className="db-studios__count">{fmtNum(s.scenes)}</span>
              </li>
            );
          })}
        </ul>
      </div>
    </DashboardCard>
  );
}

// ---- LibraryHealth ----
interface HealthItem {
  label: string;
  hint: string;
  severity: "high" | "med" | "low";
  count: number;
  linkTo: string;
}

function LibraryHealth({ health, totalScenes }: { health: DHealthCounts; totalScenes: number }) {
  const history = useHistory();

  const items: HealthItem[] = [
    { label: "Missing files",      hint: "Tracked but not on disk",    severity: "high", count: health.noFiles,      linkTo: "/scenes" },
    { label: "Duplicate scenes",   hint: "Matched by phash",           severity: "med",  count: health.duplicates,   linkTo: "/sceneDuplicateChecker" },
    { label: "Low-res scenes",     hint: "Below 720p",                 severity: "low",  count: health.lowRes,       linkTo: "/scenes" },
    { label: "Missing performers", hint: "No performer assigned",      severity: "low",  count: health.noPerformers, linkTo: `/scenes?c=("type":"performers","modifier":"IS_NULL")&sortby=date` },
    { label: "Untagged scenes",    hint: "Zero tags assigned",         severity: "low",  count: health.untagged,     linkTo: "/scenes" },
    { label: "Pending phashes",    hint: "Awaiting hash generation",   severity: "med",  count: health.missingPhash, linkTo: "/scenes" },
  ];

  // Score: average per-category health percentage (avoids double-counting overlap)
  const total = Math.max(totalScenes, 1);
  const score = clamp(
    Math.round(
      items.reduce((sum, it) => sum + 100 * (1 - it.count / total), 0) /
        items.length
    ),
    0,
    100
  );
  const scoreClass = score > 80 ? "good" : score > 60 ? "med" : "bad";

  return (
    <DashboardCard
      title="Library Health"
      right={
        <div className={`db-health-score ${scoreClass}`}>
          <span className="db-health-score__num">{score}</span>
          <span className="db-health-score__lbl">/100</span>
        </div>
      }
    >
      <ul className="db-health">
        {items.map((it) => (
          <li key={it.label} className="db-health__row">
            <span className={`db-health__dot db-health__dot--${it.severity}`} />
            <div className="db-health__label">
              {it.label}
              <span className="db-health__info">
                <svg width="11" height="11" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="8" cy="8" r="7" />
                  <line x1="8" y1="7" x2="8" y2="12" />
                  <circle cx="8" cy="4.5" r="0.5" fill="currentColor" stroke="none" />
                </svg>
                <span className="db-health__tooltip">{it.hint}</span>
              </span>
            </div>
            <span className="db-health__count">{fmtNum(it.count)}</span>
            <button
              className="db-health__cta"
              onClick={() => history.push(it.linkTo)}
            >
              Review →
            </button>
          </li>
        ))}
      </ul>
    </DashboardCard>
  );
}

// ---- TagCloud ----
function TagCloud({ tags, ready }: { tags: DTag[]; ready: boolean }) {
  const [query, setQuery] = useState("");
  const history = useHistory();

  const filtered = tags.filter((t) =>
    t.name.toLowerCase().includes(query.toLowerCase())
  );
  const maxCount = Math.max(...tags.map((t) => t.scene_count), 1);
  const minCount = Math.min(...tags.map((t) => t.scene_count), 0);

  function tagSize(count: number): number {
    if (maxCount === minCount) return 14;
    const t = (count - minCount) / (maxCount - minCount);
    return 11 + t * 9;
  }

  return (
    <DashboardCard
      title="Tags"
      right={
        <input
          className="db-tagcloud__search"
          placeholder="Filter…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      }
    >
      {!ready ? (
        <LoadingIndicator small />
      ) : (
        <div className="db-tagcloud">
          {filtered.map((t) => (
            <button
              key={t.id}
              className="db-tag"
              style={{ fontSize: tagSize(t.scene_count) + "px" }}
              onClick={() => history.push(`/tags/${t.id}/scenes`)}
            >
              {t.name}
              <span className="db-tag__count">{fmtNum(t.scene_count)}</span>
            </button>
          ))}
          {filtered.length === 0 && query && (
            <div className="db-tagcloud__empty">
              No tags match &ldquo;{query}&rdquo;.
            </div>
          )}
        </div>
      )}
    </DashboardCard>
  );
}

// ---- Dashboard (main) ----
export const Dashboard: React.FC = () => {
  const dashData = useDashboardData();

  const sparkAdded = useMemo(
    () => bucketByDay(dashData.addedScenes.map((s) => s.created_at), 90).slice(-30),
    [dashData.addedScenes]
  );
  const sparkPlays = useMemo(
    () =>
      bucketByDay(
        dashData.playedScenes.map((s) => s.last_played_at ?? ""),
        90
      ).slice(-30),
    [dashData.playedScenes]
  );

  const totalScenes = dashData.stats?.scene_count ?? 0;

  return (
    <div className="dashboard-page">
      <div className="db-page">
        <header className="db-page__head">
          <div>
            <h1 className="db-page__title">Library Overview</h1>
          </div>
        </header>

        <StatGrid
          stats={dashData.stats}
          sparkAdded={sparkAdded}
          sparkPlays={sparkPlays}
        />

        <div className="db-grid">
          <div className="db-col">
            <ActivityChart
              addedScenes={dashData.addedScenes}
              playedScenes={dashData.playedScenes}
            />
            <TopPerformers performers={dashData.performers} ready={dashData.ready} />
            <TagCloud tags={dashData.tags} ready={dashData.ready} />
          </div>
          <div className="db-col">
            <LibraryHealth health={dashData.health} totalScenes={totalScenes} />
            <TopStudios
              studios={dashData.studios}
              totalScenes={totalScenes}
              ready={dashData.ready}
            />
          </div>
        </div>
      </div>
    </div>
  );
};

export default Dashboard;
