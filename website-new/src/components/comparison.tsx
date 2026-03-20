import { useEffect, useRef, useState } from "react";
import { Check, X, Minus } from "lucide-react";
import { cn } from "@/lib/utils";

type CellValue = "yes" | "no" | "partial" | string;

interface ComparisonRow {
  feature: string;
  olb: CellValue;
  nginx: CellValue;
  haproxy: CellValue;
  traefik: CellValue;
  envoy: CellValue;
}

const rows: ComparisonRow[] = [
  {
    feature: "Zero Dependencies",
    olb: "yes",
    nginx: "no",
    haproxy: "no",
    traefik: "no",
    envoy: "no",
  },
  {
    feature: "Single Binary",
    olb: "yes",
    nginx: "no",
    haproxy: "no",
    traefik: "yes",
    envoy: "no",
  },
  {
    feature: "L4 + L7 Proxy",
    olb: "yes",
    nginx: "yes",
    haproxy: "yes",
    traefik: "yes",
    envoy: "yes",
  },
  {
    feature: "14 LB Algorithms",
    olb: "yes",
    nginx: "partial",
    haproxy: "partial",
    traefik: "partial",
    envoy: "yes",
  },
  {
    feature: "Built-in WAF",
    olb: "yes",
    nginx: "no",
    haproxy: "no",
    traefik: "no",
    envoy: "no",
  },
  {
    feature: "Auto TLS / ACME",
    olb: "yes",
    nginx: "no",
    haproxy: "no",
    traefik: "yes",
    envoy: "no",
  },
  {
    feature: "WebSocket + gRPC + SSE",
    olb: "yes",
    nginx: "yes",
    haproxy: "partial",
    traefik: "yes",
    envoy: "yes",
  },
  {
    feature: "Raft Clustering",
    olb: "yes",
    nginx: "no",
    haproxy: "no",
    traefik: "partial",
    envoy: "partial",
  },
  {
    feature: "Hot Config Reload",
    olb: "yes",
    nginx: "yes",
    haproxy: "yes",
    traefik: "yes",
    envoy: "yes",
  },
  {
    feature: "Web Dashboard",
    olb: "yes",
    nginx: "partial",
    haproxy: "no",
    traefik: "yes",
    envoy: "no",
  },
  {
    feature: "MCP / AI Integration",
    olb: "yes",
    nginx: "no",
    haproxy: "no",
    traefik: "no",
    envoy: "no",
  },
  {
    feature: "Embedded Library",
    olb: "yes",
    nginx: "no",
    haproxy: "no",
    traefik: "no",
    envoy: "no",
  },
  {
    feature: "Circuit Breaker",
    olb: "yes",
    nginx: "no",
    haproxy: "no",
    traefik: "yes",
    envoy: "yes",
  },
  {
    feature: "Passive Health Checks",
    olb: "yes",
    nginx: "no",
    haproxy: "yes",
    traefik: "no",
    envoy: "yes",
  },
];

const competitors = ["OpenLoadBalancer", "Nginx", "HAProxy", "Traefik", "Envoy"];

function StatusIcon({ value }: { value: CellValue }) {
  if (value === "yes") {
    return (
      <span className="inline-flex items-center justify-center w-6 h-6 rounded-full bg-success/10">
        <Check className="w-4 h-4 text-success" />
      </span>
    );
  }
  if (value === "no") {
    return (
      <span className="inline-flex items-center justify-center w-6 h-6 rounded-full bg-danger/10">
        <X className="w-4 h-4 text-danger" />
      </span>
    );
  }
  return (
    <span className="inline-flex items-center justify-center w-6 h-6 rounded-full bg-warning/10">
      <Minus className="w-4 h-4 text-warning" />
    </span>
  );
}

export function Comparison() {
  const ref = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(true);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setVisible(true);
          observer.disconnect();
        }
      },
      { threshold: 0.05 }
    );

    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  return (
    <section id="comparison" className="relative py-20 sm:py-28">
      {/* Background accent */}
      <div className="absolute inset-0 -z-10">
        <div className="absolute top-1/3 right-1/4 w-[500px] h-[500px] bg-secondary/5 rounded-full blur-3xl" />
      </div>

      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        {/* Section header */}
        <div className="text-center max-w-2xl mx-auto mb-16">
          <h2 className="text-3xl sm:text-4xl font-bold tracking-tight text-foreground mb-4">
            How OpenLoadBalancer{" "}
            <span className="bg-gradient-to-r from-primary to-secondary bg-clip-text text-transparent">
              compares
            </span>
          </h2>
          <p className="text-lg text-muted-foreground leading-relaxed">
            A feature-by-feature comparison with popular load balancers and
            reverse proxies.
          </p>
        </div>

        {/* Table container */}
        <div
          ref={ref}
          className={cn(
            "relative overflow-x-auto rounded-2xl border border-border bg-card/50 backdrop-blur-sm transition-all duration-700",
            visible
              ? "opacity-100 translate-y-0"
              : "opacity-0 translate-y-8"
          )}
        >
          <table className="w-full text-sm">
            <thead>
              <tr className="sticky top-0 z-10 bg-muted/80 backdrop-blur-sm border-b border-border">
                <th className="text-left font-semibold text-foreground px-6 py-4 min-w-[200px]">
                  Feature
                </th>
                {competitors.map((name, i) => (
                  <th
                    key={name}
                    className={cn(
                      "text-center font-semibold px-4 py-4 min-w-[130px]",
                      i === 0
                        ? "text-primary-foreground bg-gradient-to-b from-primary to-secondary"
                        : "text-foreground"
                    )}
                  >
                    {name}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((row, rowIndex) => {
                const cells: CellValue[] = [
                  row.olb,
                  row.nginx,
                  row.haproxy,
                  row.traefik,
                  row.envoy,
                ];
                return (
                  <tr
                    key={row.feature}
                    className={cn(
                      "border-b border-border/50 transition-colors hover:bg-muted/30",
                      rowIndex % 2 === 1 && "bg-muted/20"
                    )}
                  >
                    <td className="px-6 py-4 font-medium text-foreground whitespace-nowrap">
                      {row.feature}
                    </td>
                    {cells.map((cell, cellIndex) => (
                      <td
                        key={cellIndex}
                        className={cn(
                          "text-center px-4 py-4",
                          cellIndex === 0 && "bg-primary/5"
                        )}
                      >
                        <div className="flex items-center justify-center">
                          <StatusIcon value={cell} />
                        </div>
                      </td>
                    ))}
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>

        {/* Legend */}
        <div className="flex flex-wrap items-center justify-center gap-6 mt-6 text-sm text-muted-foreground">
          <div className="flex items-center gap-2">
            <StatusIcon value="yes" />
            <span>Supported</span>
          </div>
          <div className="flex items-center gap-2">
            <StatusIcon value="partial" />
            <span>Partial / Plugin</span>
          </div>
          <div className="flex items-center gap-2">
            <StatusIcon value="no" />
            <span>Not available</span>
          </div>
        </div>
      </div>
    </section>
  );
}
