import { ExternalLink } from "lucide-react";
import { cn } from "@/lib/utils";

const productLinks = [
  { label: "Features", href: "#features" },
  { label: "Architecture", href: "#architecture" },
  { label: "Performance", href: "#performance" },
  { label: "Comparison", href: "#comparison" },
];

const docsLinks = [
  { label: "Getting Started", href: "#quick-start" },
  { label: "Configuration", href: "#quick-start" },
  { label: "Algorithms", href: "#features" },
  { label: "API Reference", href: "https://github.com/openloadbalancer/olb/tree/main/docs" },
];

const communityLinks = [
  {
    label: "GitHub",
    href: "https://github.com/openloadbalancer/olb",
    external: true,
  },
  {
    label: "Issues",
    href: "https://github.com/openloadbalancer/olb/issues",
    external: true,
  },
  {
    label: "Releases",
    href: "https://github.com/openloadbalancer/olb/releases",
    external: true,
  },
  {
    label: "Discussions",
    href: "https://github.com/openloadbalancer/olb/discussions",
    external: true,
  },
];

function FooterColumn({
  title,
  links,
}: {
  title: string;
  links: { label: string; href: string; external?: boolean }[];
}) {
  return (
    <div>
      <h3 className="text-sm font-semibold text-foreground mb-4">{title}</h3>
      <ul className="space-y-3">
        {links.map((link) => (
          <li key={link.label}>
            <a
              href={link.href}
              {...(link.external
                ? { target: "_blank", rel: "noopener noreferrer" }
                : {})}
              className="group inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              {link.label}
              {link.external && (
                <ExternalLink className="w-3 h-3 opacity-0 -translate-y-0.5 group-hover:opacity-100 group-hover:translate-y-0 transition-all" />
              )}
            </a>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function Footer() {
  return (
    <footer className="relative border-t border-border">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        {/* Main footer grid */}
        <div className="py-12 sm:py-16 grid grid-cols-2 sm:grid-cols-4 gap-8 lg:gap-12">
          {/* Brand column */}
          <div className="col-span-2 sm:col-span-1">
            <a href="/" className="inline-flex items-center gap-2.5 mb-4">
              <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary to-secondary flex items-center justify-center">
                <span className="text-white font-bold text-xs">OLB</span>
              </div>
              <span className="font-semibold text-foreground">
                OpenLoadBalancer
              </span>
            </a>
            <p className="text-sm text-muted-foreground leading-relaxed max-w-xs">
              Zero-dependency load balancer. One binary. Total control.
            </p>
          </div>

          {/* Link columns */}
          <FooterColumn title="Product" links={productLinks} />
          <FooterColumn title="Documentation" links={docsLinks} />
          <FooterColumn title="Community" links={communityLinks} />
        </div>

        {/* Bottom bar */}
        <div className="border-t border-border py-6 flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-muted-foreground">
          <p>&copy; 2026 ECOSTACK TECHNOLOGY OÜ. All rights reserved.</p>
          <p>
            Released under the{" "}
            <a
              href="https://github.com/openloadbalancer/olb/blob/main/LICENSE"
              target="_blank"
              rel="noopener noreferrer"
              className="text-foreground hover:text-primary transition-colors"
            >
              Apache-2.0 License
            </a>
            .
          </p>
        </div>
      </div>
    </footer>
  );
}
