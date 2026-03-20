import { useState, useEffect, useRef, useCallback } from "react";
import { Check, Copy } from "lucide-react";
import { cn } from "@/lib/utils";

/* ------------------------------------------------------------------ */
/*  Tab content definitions                                            */
/* ------------------------------------------------------------------ */

interface TabDef {
  id: string;
  label: string;
  lang: string;
  langLabel: string;
  code: string;
}

const tabs: TabDef[] = [
  {
    id: "binary",
    label: "Binary",
    lang: "bash",
    langLabel: "bash",
    code: `# Download the latest release
curl -sL https://github.com/openloadbalancer/olb/releases/latest/download/olb-linux-amd64 -o olb
chmod +x olb

# Start with inline config
./olb start --config olb.yaml`,
  },
  {
    id: "docker",
    label: "Docker",
    lang: "bash",
    langLabel: "bash",
    code: `# Run with Docker
docker run -d \\
  -p 8080:8080 \\
  -p 9090:9090 \\
  -v $(pwd)/olb.yaml:/etc/olb/olb.yaml \\
  openloadbalancer/olb:latest`,
  },
  {
    id: "config",
    label: "Config",
    lang: "yaml",
    langLabel: "olb.yaml",
    code: `listeners:
  - name: http
    address: ":8080"
    protocol: http
    routes:
      - path: /
        pool: my-backends

pools:
  - name: my-backends
    algorithm: round_robin
    backends:
      - address: "10.0.1.1:3000"
      - address: "10.0.1.2:3000"
      - address: "10.0.1.3:3000"

admin:
  address: ":9090"`,
  },
  {
    id: "cluster",
    label: "Cluster",
    lang: "yaml",
    langLabel: "olb.yaml",
    code: `cluster:
  enabled: true
  node_id: olb-1
  bind_addr: "10.0.0.1"
  bind_port: 7946
  peers:
    - "10.0.0.2:7946"
    - "10.0.0.3:7946"`,
  },
];

/* ------------------------------------------------------------------ */
/*  Syntax highlighting (lightweight, no deps)                         */
/* ------------------------------------------------------------------ */

function highlightBash(code: string): React.ReactNode[] {
  return code.split("\n").map((line, i) => {
    // Comments
    if (line.trimStart().startsWith("#")) {
      return (
        <span key={i} className="text-muted-foreground/50">
          {line}
          {"\n"}
        </span>
      );
    }

    const parts: React.ReactNode[] = [];
    let rest = line;

    // Inline comment
    const commentIdx = rest.indexOf(" #");
    let comment = "";
    if (commentIdx !== -1) {
      comment = rest.slice(commentIdx);
      rest = rest.slice(0, commentIdx);
    }

    // Tokenize very lightly
    const tokens = rest.split(/(\s+)/);
    let isFirstWord = true;

    tokens.forEach((token, j) => {
      // Whitespace
      if (/^\s+$/.test(token)) {
        parts.push(token);
        return;
      }

      // Flags (--something, -x)
      if (/^-{1,2}[\w-]+$/.test(token)) {
        parts.push(
          <span key={`${i}-${j}`} className="text-accent">
            {token}
          </span>
        );
        isFirstWord = false;
        return;
      }

      // Commands (first real word on a line, or after pipe/&&)
      if (isFirstWord && /^[a-z./]/.test(token)) {
        parts.push(
          <span key={`${i}-${j}`} className="text-primary">
            {token}
          </span>
        );
        isFirstWord = false;
        return;
      }

      // Strings
      if (/^["']/.test(token)) {
        parts.push(
          <span key={`${i}-${j}`} className="text-success">
            {token}
          </span>
        );
        isFirstWord = false;
        return;
      }

      // Variables / substitutions
      if (/\$/.test(token)) {
        parts.push(
          <span key={`${i}-${j}`} className="text-warning">
            {token}
          </span>
        );
        isFirstWord = false;
        return;
      }

      // Backslash continuation
      if (token === "\\") {
        parts.push(
          <span key={`${i}-${j}`} className="text-muted-foreground/50">
            {token}
          </span>
        );
        return;
      }

      parts.push(token);
      isFirstWord = false;
    });

    if (comment) {
      parts.push(
        <span className="text-muted-foreground/50">{comment}</span>
      );
    }

    return (
      <span key={i}>
        {parts}
        {"\n"}
      </span>
    );
  });
}

function highlightYaml(code: string): React.ReactNode[] {
  return code.split("\n").map((line, i) => {
    // Comments
    if (line.trimStart().startsWith("#")) {
      return (
        <span key={i} className="text-muted-foreground/50">
          {line}
          {"\n"}
        </span>
      );
    }

    // Key: value lines
    const keyMatch = line.match(/^(\s*)([\w_-]+)(:)(.*)/);
    if (keyMatch) {
      const [, indent, key, colon, value] = keyMatch;

      let valuePart: React.ReactNode = value;

      // Quoted strings
      if (/^\s*".*"/.test(value) || /^\s*'.*'/.test(value)) {
        valuePart = <span className="text-success">{value}</span>;
      }
      // Numbers
      else if (/^\s*\d+/.test(value)) {
        valuePart = <span className="text-warning">{value}</span>;
      }
      // Booleans
      else if (/^\s*(true|false)/.test(value)) {
        valuePart = <span className="text-warning">{value}</span>;
      }

      return (
        <span key={i}>
          {indent}
          <span className="text-primary">{key}</span>
          <span className="text-muted-foreground">{colon}</span>
          {valuePart}
          {"\n"}
        </span>
      );
    }

    // List items with - "value"
    const listMatch = line.match(/^(\s*)(- )(.*)/);
    if (listMatch) {
      const [, indent, dash, value] = listMatch;
      let valuePart: React.ReactNode = value;

      if (/^".*"/.test(value) || /^'.*'/.test(value)) {
        valuePart = <span className="text-success">{value}</span>;
      } else if (/^[\w_-]+:/.test(value)) {
        // Inline key:value in list item
        const inlineMatch = value.match(/^([\w_-]+)(:)(.*)/);
        if (inlineMatch) {
          const [, k, c, v] = inlineMatch;
          let vPart: React.ReactNode = v;
          if (/^\s*".*"/.test(v) || /^\s*'.*'/.test(v)) {
            vPart = <span className="text-success">{v}</span>;
          } else if (/^\s*\d+/.test(v)) {
            vPart = <span className="text-warning">{v}</span>;
          }
          valuePart = (
            <>
              <span className="text-primary">{k}</span>
              <span className="text-muted-foreground">{c}</span>
              {vPart}
            </>
          );
        }
      }

      return (
        <span key={i}>
          {indent}
          <span className="text-muted-foreground">{dash}</span>
          {valuePart}
          {"\n"}
        </span>
      );
    }

    return (
      <span key={i}>
        {line}
        {"\n"}
      </span>
    );
  });
}

function highlight(code: string, lang: string): React.ReactNode[] {
  if (lang === "bash") return highlightBash(code);
  if (lang === "yaml") return highlightYaml(code);
  return code.split("\n").map((line, i) => (
    <span key={i}>
      {line}
      {"\n"}
    </span>
  ));
}

/* ------------------------------------------------------------------ */
/*  Code Block                                                         */
/* ------------------------------------------------------------------ */

function CodeBlock({ tab }: { tab: TabDef }) {
  const [copied, setCopied] = useState(true);

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(tab.code).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }, [tab.code]);

  const highlighted = highlight(tab.code, tab.lang);

  return (
    <div className="rounded-xl border border-border overflow-hidden bg-[#0d1117]">
      {/* Terminal header bar */}
      <div className="flex items-center justify-between px-4 py-2.5 bg-[#161b22] border-b border-[#30363d]">
        <div className="flex items-center gap-3">
          {/* Traffic light dots */}
          <div className="flex items-center gap-1.5">
            <div className="w-3 h-3 rounded-full bg-[#ff5f57]" />
            <div className="w-3 h-3 rounded-full bg-[#febc2e]" />
            <div className="w-3 h-3 rounded-full bg-[#28c840]" />
          </div>
          <span className="text-xs text-[#8b949e] font-mono">
            {tab.langLabel}
          </span>
        </div>

        {/* Copy button */}
        <button
          onClick={handleCopy}
          className={cn(
            "flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium transition-all duration-200",
            copied
              ? "bg-success/15 text-success"
              : "text-[#8b949e] hover:text-[#c9d1d9] hover:bg-[#30363d]"
          )}
        >
          {copied ? (
            <>
              <Check className="w-3.5 h-3.5" />
              Copied
            </>
          ) : (
            <>
              <Copy className="w-3.5 h-3.5" />
              Copy
            </>
          )}
        </button>
      </div>

      {/* Code content */}
      <div className="overflow-x-auto">
        <pre className="px-4 py-4 text-sm leading-relaxed">
          <code className="text-[#c9d1d9] font-mono">{highlighted}</code>
        </pre>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Main Section                                                       */
/* ------------------------------------------------------------------ */

export function QuickStart() {
  const [activeTab, setActiveTab] = useState("binary");
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

  const currentTab = tabs.find((t) => t.id === activeTab) ?? tabs[0];

  return (
    <section id="quick-start" className="relative py-20 sm:py-28">
      {/* Background accent */}
      <div className="absolute inset-0 -z-10">
        <div className="absolute bottom-0 right-1/4 w-[500px] h-[500px] bg-primary/5 rounded-full blur-3xl" />
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
            Up and running{" "}
            <span className="bg-gradient-to-r from-primary to-secondary bg-clip-text text-transparent">
              in minutes
            </span>
          </h2>
          <p className="text-lg text-muted-foreground leading-relaxed">
            Download a single binary and start load balancing. No complex setup,
            no configuration headaches.
          </p>
        </div>

        {/* Code block card */}
        <div
          className={cn(
            "max-w-3xl mx-auto transition-all duration-700 delay-200",
            visible
              ? "opacity-100 translate-y-0"
              : "opacity-0 translate-y-8"
          )}
        >
          {/* Tab buttons */}
          <div className="flex items-center gap-1 mb-4 p-1 rounded-xl bg-muted/50 border border-border w-fit">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={cn(
                  "px-4 py-2 rounded-lg text-sm font-medium transition-all duration-200",
                  activeTab === tab.id
                    ? "bg-background text-foreground shadow-sm border border-border"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                {tab.label}
              </button>
            ))}
          </div>

          {/* Active code block */}
          <CodeBlock tab={currentTab} />
        </div>
      </div>
    </section>
  );
}
