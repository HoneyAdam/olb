import { useState, useEffect, useRef, Fragment } from "react";
import {
  ShieldAlert,
  Cog,
  BarChart3,
  ArrowRight,
} from "lucide-react";
import { cn } from "@/lib/utils";

const categories = [
  {
    name: "Security",
    color: "danger" as const,
    icon: ShieldAlert,
    layers: [
      {
        num: "01",
        name: "Recovery",
        description:
          "Catches panics from downstream handlers, returns 500 instead of crashing",
      },
      {
        num: "02",
        name: "Body Limit",
        description:
          "Rejects oversized request bodies before they reach the pipeline",
      },
      {
        num: "03",
        name: "IP Filter",
        description:
          "Whitelist/blacklist IP filtering with CIDR range support",
      },
      {
        num: "04",
        name: "WAF",
        description:
          "6-layer security: IP ACL, rate limit, sanitizer, detection, bot, response",
      },
      {
        num: "05",
        name: "Real IP",
        description:
          "Extracts real client IP from X-Forwarded-For and X-Real-IP headers",
      },
    ],
  },
  {
    name: "Processing",
    color: "primary" as const,
    icon: Cog,
    layers: [
      {
        num: "06",
        name: "Request ID",
        description:
          "Generates unique request ID for distributed tracing and log correlation",
      },
      {
        num: "07",
        name: "Timeout",
        description:
          "Enforces maximum request processing time, returns 504 on timeout",
      },
      {
        num: "08",
        name: "Rate Limiter",
        description:
          "Token bucket per-client rate limiting with X-Forwarded-For support",
      },
      {
        num: "09",
        name: "Circuit Breaker",
        description:
          "Three-state (closed/open/half-open) circuit breaker per backend",
      },
      {
        num: "10",
        name: "CORS",
        description:
          "Origin validation, preflight handling, credential support",
      },
      {
        num: "11",
        name: "Headers",
        description:
          "Request/response header manipulation and injection",
      },
      {
        num: "12",
        name: "Compression",
        description:
          "gzip/deflate compression with Accept-Encoding negotiation",
      },
    ],
  },
  {
    name: "Observability",
    color: "success" as const,
    icon: BarChart3,
    layers: [
      {
        num: "13",
        name: "Retry",
        description:
          "Automatic retry for idempotent methods with exponential backoff",
      },
      {
        num: "14",
        name: "Cache",
        description:
          "In-memory response caching with TTL and cache-control support",
      },
      {
        num: "15",
        name: "Metrics",
        description:
          "Prometheus-compatible metrics collection per route/method/status",
      },
      {
        num: "16",
        name: "Access Log",
        description:
          "Structured JSON access logging with latency, backend, bytes in/out",
      },
    ],
  },
];

const flowSteps = [
  { label: "Request", dotClass: "bg-foreground/20 border-foreground/40" },
  { label: "Security", dotClass: "bg-danger/30 border-danger/60", textClass: "text-danger" },
  { label: "Processing", dotClass: "bg-primary/30 border-primary/60", textClass: "text-primary" },
  { label: "Observability", dotClass: "bg-success/30 border-success/60", textClass: "text-success" },
  { label: "Backend", dotClass: "bg-foreground/20 border-foreground/40" },
];

const lineGradients = [
  "from-foreground/10 to-danger/30",
  "from-danger/30 to-primary/30",
  "from-primary/30 to-success/30",
  "from-success/30 to-foreground/10",
];

const styles = {
  danger: {
    dot: "bg-danger",
    text: "text-danger",
    iconBg: "bg-danger/10",
    headerLine: "from-danger/50 to-transparent",
    badge: "bg-danger/10 text-danger border-danger/20",
    badgeActive: "bg-danger/20 text-danger border-danger/40",
    cardActive:
      "border-danger/30 bg-gradient-to-r from-danger/[0.07] to-transparent shadow-lg shadow-danger/[0.06]",
    columnBorder: "border-danger/15 hover:border-danger/25",
  },
  primary: {
    dot: "bg-primary",
    text: "text-primary",
    iconBg: "bg-primary/10",
    headerLine: "from-primary/50 to-transparent",
    badge: "bg-primary/10 text-primary border-primary/20",
    badgeActive: "bg-primary/20 text-primary border-primary/40",
    cardActive:
      "border-primary/30 bg-gradient-to-r from-primary/[0.07] to-transparent shadow-lg shadow-primary/[0.06]",
    columnBorder: "border-primary/15 hover:border-primary/25",
  },
  success: {
    dot: "bg-success",
    text: "text-success",
    iconBg: "bg-success/10",
    headerLine: "from-success/50 to-transparent",
    badge: "bg-success/10 text-success border-success/20",
    badgeActive: "bg-success/20 text-success border-success/40",
    cardActive:
      "border-success/30 bg-gradient-to-r from-success/[0.07] to-transparent shadow-lg shadow-success/[0.06]",
    columnBorder: "border-success/15 hover:border-success/25",
  },
};

export function Pipeline() {
  const [activeLayer, setActiveLayer] = useState<string | null>(null);
  const sectionRef = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false);

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

  // Which category column is active (for flow indicator highlight)
  const activeCatIdx = activeLayer
    ? categories.findIndex((c) =>
        c.layers.some((l) => l.num === activeLayer)
      )
    : -1;

  return (
    <section id="architecture" className="relative py-20 sm:py-28">
      {/* Background accent */}
      <div className="absolute inset-0 -z-10">
        <div className="absolute top-0 right-0 w-[500px] h-[500px] bg-primary/5 rounded-full blur-3xl" />
        <div className="absolute bottom-0 left-0 w-[400px] h-[400px] bg-secondary/5 rounded-full blur-3xl" />
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
            16-Layer{" "}
            <span className="bg-gradient-to-r from-primary to-secondary bg-clip-text text-transparent">
              Middleware Pipeline
            </span>
          </h2>
          <p className="text-lg text-muted-foreground leading-relaxed">
            Every request flows through a configurable middleware chain. Each
            layer can be independently enabled, disabled, or customized.
          </p>
        </div>

        {/* Flow indicator — desktop */}
        <div
          className={cn(
            "hidden sm:flex items-center justify-between max-w-3xl mx-auto mb-12 transition-all duration-700 delay-100",
            visible ? "opacity-100" : "opacity-0"
          )}
        >
          {flowSteps.map((step, i) => (
            <Fragment key={step.label}>
              {/* Node */}
              <div
                className={cn(
                  "flex flex-col items-center gap-1.5 transition-transform duration-300",
                  i > 0 &&
                    i < 4 &&
                    activeCatIdx === i - 1 &&
                    "scale-110"
                )}
              >
                <div
                  className={cn(
                    "w-3 h-3 rounded-full border-2 transition-all duration-300",
                    step.dotClass,
                    i > 0 &&
                      i < 4 &&
                      activeCatIdx === i - 1 &&
                      "scale-125"
                  )}
                />
                <span
                  className={cn(
                    "text-[11px] font-semibold tracking-wide",
                    step.textClass ?? "text-muted-foreground"
                  )}
                >
                  {step.label}
                </span>
              </div>

              {/* Connecting line */}
              {i < flowSteps.length - 1 && (
                <div className="flex-1 mx-3 h-px relative overflow-hidden">
                  <div
                    className={cn(
                      "absolute inset-0 bg-gradient-to-r",
                      lineGradients[i]
                    )}
                  />
                  {/* Shimmer */}
                  <div
                    className="absolute inset-y-0 w-1/3 bg-gradient-to-r from-transparent via-white/[0.12] to-transparent pipeline-shimmer"
                    style={{ animationDelay: `${i * 0.6}s` }}
                  />
                </div>
              )}
            </Fragment>
          ))}
        </div>

        {/* Mobile flow indicator */}
        <div
          className={cn(
            "flex sm:hidden items-center justify-center gap-2 mb-8 transition-all duration-700 delay-100",
            visible ? "opacity-100" : "opacity-0"
          )}
        >
          <span className="text-xs font-medium text-muted-foreground">
            Request
          </span>
          <ArrowRight className="w-3 h-3 text-muted-foreground/40" />
          <span className="text-xs font-semibold text-danger">Security</span>
          <ArrowRight className="w-3 h-3 text-muted-foreground/40" />
          <span className="text-xs font-semibold text-primary">Processing</span>
          <ArrowRight className="w-3 h-3 text-muted-foreground/40" />
          <span className="text-xs font-semibold text-success">Observability</span>
          <ArrowRight className="w-3 h-3 text-muted-foreground/40" />
          <span className="text-xs font-medium text-muted-foreground">
            Backend
          </span>
        </div>

        {/* Three category columns */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 lg:gap-5">
          {categories.map((category, catIdx) => {
            const s = styles[category.color];
            const Icon = category.icon;
            return (
              <div
                key={category.name}
                className={cn(
                  "rounded-2xl border bg-card/50 backdrop-blur-sm overflow-hidden transition-all duration-700",
                  s.columnBorder,
                  visible
                    ? "opacity-100 translate-y-0"
                    : "opacity-0 translate-y-8"
                )}
                style={{ transitionDelay: `${catIdx * 150 + 200}ms` }}
              >
                {/* Category header */}
                <div className="px-5 pt-5 pb-3">
                  <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center gap-2.5">
                      <div
                        className={cn(
                          "w-7 h-7 rounded-lg flex items-center justify-center",
                          s.iconBg
                        )}
                      >
                        <Icon className={cn("w-4 h-4", s.text)} />
                      </div>
                      <h3
                        className={cn(
                          "text-sm font-bold uppercase tracking-wider",
                          s.text
                        )}
                      >
                        {category.name}
                      </h3>
                    </div>
                    <span className="text-[11px] text-muted-foreground/50 font-medium tabular-nums">
                      {category.layers.length} layers
                    </span>
                  </div>
                  <div
                    className={cn("h-px bg-gradient-to-r", s.headerLine)}
                  />
                </div>

                {/* Layer list */}
                <div className="px-3 pb-4 space-y-1">
                  {category.layers.map((layer) => {
                    const isActive = activeLayer === layer.num;
                    return (
                      <button
                        key={layer.num}
                        onMouseEnter={() => setActiveLayer(layer.num)}
                        onMouseLeave={() => setActiveLayer(null)}
                        onClick={() =>
                          setActiveLayer(
                            activeLayer === layer.num ? null : layer.num
                          )
                        }
                        className={cn(
                          "w-full text-left px-3 py-2.5 rounded-xl border transition-all duration-200 cursor-pointer select-none group",
                          isActive
                            ? s.cardActive
                            : "border-transparent hover:bg-muted/40"
                        )}
                      >
                        <div className="flex items-center gap-3">
                          <span
                            className={cn(
                              "shrink-0 w-7 h-7 rounded-lg border flex items-center justify-center font-mono text-[11px] font-bold transition-all duration-200",
                              isActive ? s.badgeActive : s.badge
                            )}
                          >
                            {layer.num}
                          </span>
                          <span
                            className={cn(
                              "text-sm font-medium transition-colors duration-200",
                              isActive
                                ? "text-foreground"
                                : "text-muted-foreground group-hover:text-foreground"
                            )}
                          >
                            {layer.name}
                          </span>
                        </div>
                        {/* Expandable description */}
                        <div
                          className={cn(
                            "overflow-hidden transition-all duration-300 ease-out",
                            isActive
                              ? "max-h-20 opacity-100 mt-2"
                              : "max-h-0 opacity-0"
                          )}
                        >
                          <p className="text-xs text-muted-foreground leading-relaxed pl-10">
                            {layer.description}
                          </p>
                        </div>
                      </button>
                    );
                  })}
                </div>
              </div>
            );
          })}
        </div>
      </div>

      <style>{`
        @keyframes pipelineShimmer {
          0% { transform: translateX(-100%); }
          100% { transform: translateX(400%); }
        }
        .pipeline-shimmer {
          animation: pipelineShimmer 3s ease-in-out infinite;
        }
      `}</style>
    </section>
  );
}
