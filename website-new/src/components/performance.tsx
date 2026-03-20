import { useEffect, useRef, useState } from "react";
import {
  Timer,
  Gauge,
  MemoryStick,
  HardDrive,
  Cpu,
  Rocket,
} from "lucide-react";
import { cn } from "@/lib/utils";

const metrics = [
  {
    icon: Timer,
    value: "<1ms",
    label: "p99 Latency",
    description: "Detection pipeline",
  },
  {
    icon: Gauge,
    value: "50K+",
    label: "Requests/sec",
    description: "Single core throughput",
  },
  {
    icon: Cpu,
    value: "0",
    label: "Allocations",
    description: "Hot path zero-alloc",
  },
  {
    icon: HardDrive,
    value: "15MB",
    label: "Binary Size",
    description: "Statically linked",
  },
  {
    icon: MemoryStick,
    value: "<16MB",
    label: "Memory",
    description: "Idle footprint",
  },
  {
    icon: Rocket,
    value: "~2s",
    label: "Cold Start",
    description: "Ready to serve",
  },
];

function MetricCard({
  metric,
  index,
  visible,
}: {
  metric: (typeof metrics)[number];
  index: number;
  visible: boolean;
}) {
  const Icon = metric.icon;

  return (
    <div
      className={cn(
        "group relative p-6 rounded-2xl border border-border bg-card/50 backdrop-blur-sm transition-all duration-500 hover:shadow-lg hover:shadow-primary/10 hover:border-primary/30 hover:scale-[1.03]",
        visible
          ? "opacity-100 translate-y-0"
          : "opacity-0 translate-y-8"
      )}
      style={{ transitionDelay: `${index * 100}ms` }}
    >
      {/* Hover glow */}
      <div className="absolute inset-0 rounded-2xl bg-gradient-to-br from-primary/5 via-transparent to-secondary/5 opacity-0 group-hover:opacity-100 transition-opacity duration-300" />

      <div className="relative flex flex-col items-center text-center">
        <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-primary/10 to-secondary/10 flex items-center justify-center mb-4 group-hover:from-primary/20 group-hover:to-secondary/20 transition-colors">
          <Icon className="w-6 h-6 text-primary" />
        </div>

        <span className="text-3xl sm:text-4xl font-bold bg-gradient-to-r from-primary to-accent bg-clip-text text-transparent mb-1">
          {metric.value}
        </span>

        <span className="text-sm font-semibold text-foreground mb-1">
          {metric.label}
        </span>

        <span className="text-xs text-muted-foreground">
          {metric.description}
        </span>
      </div>
    </div>
  );
}

export function Performance() {
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
      { threshold: 0.1 }
    );

    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  return (
    <section id="performance" className="relative py-20 sm:py-28">
      {/* Background accent */}
      <div className="absolute inset-0 -z-10">
        <div className="absolute bottom-1/3 left-1/3 w-[600px] h-[600px] bg-accent/5 rounded-full blur-3xl" />
        <div className="absolute top-1/4 right-1/4 w-[400px] h-[400px] bg-primary/5 rounded-full blur-3xl" />
      </div>

      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        {/* Section header */}
        <div className="text-center max-w-2xl mx-auto mb-16">
          <h2 className="text-3xl sm:text-4xl font-bold tracking-tight text-foreground mb-4">
            Built for{" "}
            <span className="bg-gradient-to-r from-primary to-accent bg-clip-text text-transparent">
              performance
            </span>
          </h2>
          <p className="text-lg text-muted-foreground leading-relaxed">
            Security and routing shouldn't slow you down. Optimized from the
            ground up with zero-allocation hot paths.
          </p>
        </div>

        {/* Metrics grid */}
        <div
          ref={ref}
          className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 sm:gap-6 max-w-4xl mx-auto"
        >
          {metrics.map((metric, i) => (
            <MetricCard
              key={metric.label}
              metric={metric}
              index={i}
              visible={visible}
            />
          ))}
        </div>
      </div>
    </section>
  );
}
