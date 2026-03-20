import { useState, useEffect, useRef } from "react";
import { cn } from "@/lib/utils";

const layers = [
  {
    num: "01",
    name: "Recovery",
    description:
      "Catches panics from all downstream handlers, returns 500 instead of crashing",
    color: "danger" as const,
  },
  {
    num: "02",
    name: "Body Limit",
    description:
      "Rejects oversized request bodies before they reach processing pipeline",
    color: "danger" as const,
  },
  {
    num: "03",
    name: "IP Filter",
    description:
      "Whitelist/blacklist IP filtering with CIDR range support",
    color: "danger" as const,
  },
  {
    num: "04",
    name: "WAF",
    description:
      "6-layer security pipeline: IP ACL, rate limit, sanitizer, detection, bot, response",
    color: "danger" as const,
  },
  {
    num: "05",
    name: "Real IP",
    description:
      "Extracts real client IP from X-Forwarded-For and X-Real-IP headers",
    color: "danger" as const,
  },
  {
    num: "06",
    name: "Request ID",
    description:
      "Generates unique request ID for distributed tracing and log correlation",
    color: "primary" as const,
  },
  {
    num: "07",
    name: "Timeout",
    description:
      "Enforces maximum request processing time, returns 504 on timeout",
    color: "primary" as const,
  },
  {
    num: "08",
    name: "Rate Limiter",
    description:
      "Token bucket per-client rate limiting with X-Forwarded-For support",
    color: "primary" as const,
  },
  {
    num: "09",
    name: "Circuit Breaker",
    description:
      "Three-state (closed/open/half-open) circuit breaker per backend",
    color: "primary" as const,
  },
  {
    num: "10",
    name: "CORS",
    description:
      "Origin validation, preflight handling, credential support",
    color: "primary" as const,
  },
  {
    num: "11",
    name: "Headers",
    description:
      "Request/response header manipulation and injection",
    color: "primary" as const,
  },
  {
    num: "12",
    name: "Compression",
    description:
      "gzip/deflate compression with Accept-Encoding negotiation",
    color: "primary" as const,
  },
  {
    num: "13",
    name: "Retry",
    description:
      "Automatic retry for idempotent methods (GET/HEAD/OPTIONS) with backoff",
    color: "success" as const,
  },
  {
    num: "14",
    name: "Cache",
    description:
      "In-memory response caching with TTL and cache-control support",
    color: "success" as const,
  },
  {
    num: "15",
    name: "Metrics",
    description:
      "Prometheus-compatible metrics collection per route/method/status",
    color: "success" as const,
  },
  {
    num: "16",
    name: "Access Log",
    description:
      "Structured JSON access logging with latency, backend, bytes in/out",
    color: "success" as const,
  },
];

const colorMap = {
  danger: {
    pill: "border-danger/30 bg-danger/10 text-danger hover:bg-danger/20 hover:border-danger/50",
    pillActive:
      "border-danger/60 bg-danger/20 text-danger shadow-lg shadow-danger/10",
    num: "text-danger/60",
    label: "Security",
    labelClass: "text-danger",
  },
  primary: {
    pill: "border-primary/30 bg-primary/10 text-primary hover:bg-primary/20 hover:border-primary/50",
    pillActive:
      "border-primary/60 bg-primary/20 text-primary shadow-lg shadow-primary/10",
    num: "text-primary/60",
    label: "Processing",
    labelClass: "text-primary",
  },
  success: {
    pill: "border-success/30 bg-success/10 text-success hover:bg-success/20 hover:border-success/50",
    pillActive:
      "border-success/60 bg-success/20 text-success shadow-lg shadow-success/10",
    num: "text-success/60",
    label: "Observability",
    labelClass: "text-success",
  },
};

export function Pipeline() {
  const [activeIndex, setActiveIndex] = useState<number | null>(null);
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

  const activeLayer = activeIndex !== null ? layers[activeIndex] : null;

  return (
    <section id="architecture" className="relative py-20 sm:py-28">
      {/* Background accent */}
      <div className="absolute inset-0 -z-10">
        <div className="absolute top-0 right-0 w-[500px] h-[500px] bg-primary/5 rounded-full blur-3xl" />
        <div className="absolute bottom-0 left-0 w-[400px] h-[400px] bg-secondary/5 rounded-full blur-3xl" />
      </div>

      <div ref={sectionRef} className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
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

        {/* Pipeline card */}
        <div
          className={cn(
            "relative rounded-2xl border border-border bg-card/50 backdrop-blur-sm p-6 sm:p-8 transition-all duration-700 delay-200",
            visible
              ? "opacity-100 translate-y-0"
              : "opacity-0 translate-y-6"
          )}
        >
          {/* Legend */}
          <div className="flex flex-wrap items-center gap-4 sm:gap-6 mb-6">
            <div className="flex items-center gap-2">
              <div className="w-3 h-3 rounded-full bg-danger/40 border border-danger/50" />
              <span className="text-xs text-muted-foreground">Security</span>
            </div>
            <div className="flex items-center gap-2">
              <div className="w-3 h-3 rounded-full bg-primary/40 border border-primary/50" />
              <span className="text-xs text-muted-foreground">Processing</span>
            </div>
            <div className="flex items-center gap-2">
              <div className="w-3 h-3 rounded-full bg-success/40 border border-success/50" />
              <span className="text-xs text-muted-foreground">
                Observability
              </span>
            </div>
          </div>

          {/* Row 1 */}
          <div className="flex flex-wrap gap-2 sm:gap-3 mb-3">
            {layers.slice(0, 8).map((layer, i) => {
              const colors = colorMap[layer.color];
              const isActive = activeIndex === i;
              return (
                <button
                  key={layer.num}
                  onMouseEnter={() => setActiveIndex(i)}
                  onMouseLeave={() => setActiveIndex(null)}
                  onClick={() =>
                    setActiveIndex(activeIndex === i ? null : i)
                  }
                  className={cn(
                    "inline-flex items-center gap-2 px-3 py-2 sm:px-4 sm:py-2.5 rounded-xl border text-sm font-medium transition-all duration-200 cursor-pointer select-none",
                    isActive ? colors.pillActive : colors.pill
                  )}
                >
                  <span
                    className={cn(
                      "font-mono text-xs",
                      isActive ? "" : colors.num
                    )}
                  >
                    {layer.num}
                  </span>
                  <span>{layer.name}</span>
                </button>
              );
            })}
          </div>

          {/* Row 2 */}
          <div className="flex flex-wrap gap-2 sm:gap-3 mb-6">
            {layers.slice(8, 16).map((layer, i) => {
              const idx = i + 8;
              const colors = colorMap[layer.color];
              const isActive = activeIndex === idx;
              return (
                <button
                  key={layer.num}
                  onMouseEnter={() => setActiveIndex(idx)}
                  onMouseLeave={() => setActiveIndex(null)}
                  onClick={() =>
                    setActiveIndex(activeIndex === idx ? null : idx)
                  }
                  className={cn(
                    "inline-flex items-center gap-2 px-3 py-2 sm:px-4 sm:py-2.5 rounded-xl border text-sm font-medium transition-all duration-200 cursor-pointer select-none",
                    isActive ? colors.pillActive : colors.pill
                  )}
                >
                  <span
                    className={cn(
                      "font-mono text-xs",
                      isActive ? "" : colors.num
                    )}
                  >
                    {layer.num}
                  </span>
                  <span>{layer.name}</span>
                </button>
              );
            })}
          </div>

          {/* Description area */}
          <div className="min-h-[72px] relative">
            {activeLayer ? (
              <div className="flex items-start gap-3 p-4 rounded-xl bg-muted/50 border border-border animate-in fade-in duration-200">
                <div
                  className={cn(
                    "shrink-0 w-8 h-8 rounded-lg flex items-center justify-center font-mono text-xs font-bold",
                    activeLayer.color === "danger" &&
                      "bg-danger/15 text-danger",
                    activeLayer.color === "primary" &&
                      "bg-primary/15 text-primary",
                    activeLayer.color === "success" &&
                      "bg-success/15 text-success"
                  )}
                >
                  {activeLayer.num}
                </div>
                <div>
                  <div className="flex items-center gap-2 mb-1">
                    <span className="text-sm font-semibold text-foreground">
                      {activeLayer.name}
                    </span>
                    <span
                      className={cn(
                        "text-[10px] font-medium uppercase tracking-wider",
                        colorMap[activeLayer.color].labelClass
                      )}
                    >
                      {colorMap[activeLayer.color].label}
                    </span>
                  </div>
                  <p className="text-sm text-muted-foreground leading-relaxed">
                    {activeLayer.description}
                  </p>
                </div>
              </div>
            ) : (
              <div className="flex items-center justify-center h-full py-4">
                <p className="text-sm text-muted-foreground/60">
                  Hover over a layer to learn more
                </p>
              </div>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}
