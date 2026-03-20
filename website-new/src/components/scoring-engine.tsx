import { useEffect, useRef, useState } from "react";
import { cn } from "@/lib/utils";

/* ------------------------------------------------------------------ */
/*  Round Robin visualization                                          */
/* ------------------------------------------------------------------ */
function RoundRobinViz() {
  const [active, setActive] = useState(0);

  useEffect(() => {
    const id = setInterval(() => setActive((p) => (p + 1) % 3), 1200);
    return () => clearInterval(id);
  }, []);

  const backends = ["srv-1", "srv-2", "srv-3"];

  return (
    <div className="flex flex-col items-center gap-3 py-4">
      {/* Request arrow */}
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <span className="font-mono">request</span>
        <svg width="24" height="12" viewBox="0 0 24 12" className="text-primary">
          <path
            d="M0 6h20m-4-4l4 4-4 4"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </div>

      {/* Backends */}
      <div className="flex items-center gap-3">
        {backends.map((name, i) => (
          <div
            key={name}
            className={cn(
              "relative px-3 py-2 rounded-lg border font-mono text-xs transition-all duration-300",
              active === i
                ? "border-primary/60 bg-primary/15 text-primary shadow-md shadow-primary/10 scale-105"
                : "border-border bg-muted/30 text-muted-foreground"
            )}
          >
            {active === i && (
              <div className="absolute -top-1.5 -right-1.5 w-3 h-3 rounded-full bg-primary animate-pulse" />
            )}
            {name}
          </div>
        ))}
      </div>

      {/* Cycling indicator */}
      <div className="flex items-center gap-1.5">
        {backends.map((_, i) => (
          <div
            key={i}
            className={cn(
              "w-1.5 h-1.5 rounded-full transition-all duration-300",
              active === i ? "bg-primary w-4" : "bg-muted-foreground/30"
            )}
          />
        ))}
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Least Connections visualization                                    */
/* ------------------------------------------------------------------ */
function LeastConnectionsViz() {
  const connections = [
    { name: "srv-1", count: 5 },
    { name: "srv-2", count: 2 },
    { name: "srv-3", count: 8 },
  ];

  const minIdx = 1; // srv-2 has the least

  return (
    <div className="flex flex-col items-center gap-3 py-4">
      {/* Request arrow */}
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <span className="font-mono">request</span>
        <svg width="24" height="12" viewBox="0 0 24 12" className="text-accent">
          <path
            d="M0 6h20m-4-4l4 4-4 4"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </div>

      {/* Backends with connection counts */}
      <div className="flex items-center gap-3">
        {connections.map((backend, i) => (
          <div
            key={backend.name}
            className={cn(
              "relative flex flex-col items-center gap-1 px-3 py-2 rounded-lg border font-mono text-xs transition-all duration-300",
              i === minIdx
                ? "border-accent/60 bg-accent/15 text-accent shadow-md shadow-accent/10 scale-105"
                : "border-border bg-muted/30 text-muted-foreground"
            )}
          >
            {i === minIdx && (
              <div className="absolute -top-1.5 -right-1.5 w-3 h-3 rounded-full bg-accent animate-pulse" />
            )}
            <span>{backend.name}</span>
            <span
              className={cn(
                "text-[10px] tabular-nums",
                i === minIdx ? "text-accent" : "text-muted-foreground/60"
              )}
            >
              {backend.count} conn
            </span>
          </div>
        ))}
      </div>

      {/* Label */}
      <div className="text-[10px] text-muted-foreground/60 font-mono">
        min(connections)
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Consistent Hash Ring visualization                                 */
/* ------------------------------------------------------------------ */
function ConsistentHashViz() {
  const [hovered, setHovered] = useState<number | null>(null);

  // Positions on a circle (in degrees), evenly spaced nodes
  const nodes = [
    { deg: 30, name: "srv-1" },
    { deg: 150, name: "srv-2" },
    { deg: 270, name: "srv-3" },
  ];

  // Virtual node / request positions
  const requests = [
    { deg: 60, target: 0 },
    { deg: 180, target: 1 },
    { deg: 310, target: 2 },
  ];

  const radius = 44;
  const cx = 56;
  const cy = 56;

  function toXY(deg: number) {
    const rad = ((deg - 90) * Math.PI) / 180;
    return {
      x: cx + radius * Math.cos(rad),
      y: cy + radius * Math.sin(rad),
    };
  }

  return (
    <div className="flex flex-col items-center gap-3 py-4">
      <svg
        width="112"
        height="112"
        viewBox="0 0 112 112"
        className="overflow-visible"
      >
        {/* Ring */}
        <circle
          cx={cx}
          cy={cy}
          r={radius}
          fill="none"
          stroke="currentColor"
          className="text-border"
          strokeWidth="1.5"
          strokeDasharray="4 3"
        />

        {/* Request dots */}
        {requests.map((req, i) => {
          const pos = toXY(req.deg);
          return (
            <circle
              key={`req-${i}`}
              cx={pos.x}
              cy={pos.y}
              r="3"
              className={cn(
                "transition-all duration-200",
                hovered === req.target
                  ? "fill-secondary opacity-100"
                  : "fill-muted-foreground opacity-40"
              )}
            />
          );
        })}

        {/* Node dots */}
        {nodes.map((node, i) => {
          const pos = toXY(node.deg);
          return (
            <g
              key={node.name}
              onMouseEnter={() => setHovered(i)}
              onMouseLeave={() => setHovered(null)}
              className="cursor-pointer"
            >
              <circle
                cx={pos.x}
                cy={pos.y}
                r="7"
                className={cn(
                  "transition-all duration-200",
                  hovered === i
                    ? "fill-secondary/30 stroke-secondary"
                    : "fill-primary/20 stroke-primary/60"
                )}
                strokeWidth="1.5"
              />
              <text
                x={pos.x}
                y={pos.y}
                textAnchor="middle"
                dominantBaseline="central"
                className="fill-foreground text-[7px] font-mono pointer-events-none"
              >
                {i + 1}
              </text>
            </g>
          );
        })}

        {/* Center label */}
        <text
          x={cx}
          y={cy}
          textAnchor="middle"
          dominantBaseline="central"
          className="fill-muted-foreground text-[7px] font-mono"
        >
          hash ring
        </text>
      </svg>

      <div className="flex items-center gap-2">
        {nodes.map((node, i) => (
          <span
            key={node.name}
            className={cn(
              "font-mono text-[10px] transition-colors duration-200",
              hovered === i ? "text-secondary" : "text-muted-foreground/60"
            )}
          >
            {node.name}
          </span>
        ))}
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Algorithm Card                                                     */
/* ------------------------------------------------------------------ */
function AlgorithmCard({
  title,
  badge,
  badgeColor,
  description,
  children,
  index,
  visible,
}: {
  title: string;
  badge: string;
  badgeColor: string;
  description: string;
  children: React.ReactNode;
  index: number;
  visible: boolean;
}) {
  return (
    <div
      className={cn(
        "group relative flex flex-col rounded-2xl border border-border bg-card/50 backdrop-blur-sm overflow-hidden transition-all duration-500 hover:border-primary/20 hover:shadow-lg hover:shadow-primary/5",
        visible ? "opacity-100 translate-y-0" : "opacity-0 translate-y-8"
      )}
      style={{ transitionDelay: `${index * 120}ms` }}
    >
      {/* Visualization area */}
      <div className="relative px-6 pt-6 pb-2 min-h-[180px] flex items-center justify-center">
        {/* Subtle grid background */}
        <div
          className="absolute inset-0 opacity-[0.03]"
          style={{
            backgroundImage:
              "linear-gradient(var(--color-muted-foreground) 1px, transparent 1px), linear-gradient(90deg, var(--color-muted-foreground) 1px, transparent 1px)",
            backgroundSize: "20px 20px",
          }}
        />
        <div className="relative">{children}</div>
      </div>

      {/* Card content */}
      <div className="px-6 pb-6 flex-1 flex flex-col">
        <div className="flex items-center gap-2.5 mb-2">
          <h3 className="text-base font-semibold text-foreground">{title}</h3>
          <span
            className={cn(
              "inline-flex items-center px-2 py-0.5 rounded-md text-[10px] font-semibold uppercase tracking-wider border",
              badgeColor
            )}
          >
            {badge}
          </span>
        </div>
        <p className="text-sm text-muted-foreground leading-relaxed">
          {description}
        </p>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Main Section                                                       */
/* ------------------------------------------------------------------ */
export function ScoringEngine() {
  const sectionRef = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(true);

  useEffect(() => {
    const el = sectionRef.current;
    if (!el) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setVisible(true);
          observer.disconnect();
        }
      },
      { threshold: 0.1 }
    );

    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  return (
    <section className="relative py-20 sm:py-28">
      {/* Background accent */}
      <div className="absolute inset-0 -z-10">
        <div className="absolute top-1/3 left-1/3 w-[500px] h-[500px] bg-secondary/5 rounded-full blur-3xl" />
      </div>

      <div
        ref={sectionRef}
        className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"
      >
        {/* Section header */}
        <div
          className={cn(
            "text-center max-w-2xl mx-auto mb-14 transition-all duration-700",
            visible
              ? "opacity-100 translate-y-0"
              : "opacity-0 translate-y-6"
          )}
        >
          <h2 className="text-3xl sm:text-4xl font-bold tracking-tight text-foreground mb-4">
            Intelligent{" "}
            <span className="bg-gradient-to-r from-primary to-secondary bg-clip-text text-transparent">
              Load Balancing
            </span>
          </h2>
          <p className="text-lg text-muted-foreground leading-relaxed">
            14 algorithms for every use case. From simple round-robin to
            consistent hashing with virtual nodes.
          </p>
        </div>

        {/* Algorithm cards */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          <AlgorithmCard
            title="Round Robin"
            badge="Default"
            badgeColor="border-primary/30 bg-primary/10 text-primary"
            description="Distributes requests evenly across all healthy backends in circular order."
            index={0}
            visible={visible}
          >
            <RoundRobinViz />
          </AlgorithmCard>

          <AlgorithmCard
            title="Least Connections"
            badge="Adaptive"
            badgeColor="border-accent/30 bg-accent/10 text-accent"
            description="Routes to the backend with fewest active connections. Optimal for varying response times."
            index={1}
            visible={visible}
          >
            <LeastConnectionsViz />
          </AlgorithmCard>

          <AlgorithmCard
            title="Consistent Hash"
            badge="Sticky"
            badgeColor="border-secondary/30 bg-secondary/10 text-secondary"
            description="Maps requests to backends via hash ring. Minimal redistribution when backends change."
            index={2}
            visible={visible}
          >
            <ConsistentHashViz />
          </AlgorithmCard>
        </div>
      </div>
    </section>
  );
}
