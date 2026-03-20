import { Header } from "@/components/header";
import { Hero } from "@/components/hero";
import { Features } from "@/components/features";
import { Pipeline } from "@/components/pipeline";
import { ScoringEngine } from "@/components/scoring-engine";
import { QuickStart } from "@/components/quick-start";
import { Comparison } from "@/components/comparison";
import { Performance } from "@/components/performance";
import { CTA } from "@/components/cta";
import { Footer } from "@/components/footer";

export function App() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <main>
        <Hero />
        <Features />
        <Pipeline />
        <ScoringEngine />
        <QuickStart />
        <Comparison />
        <Performance />
        <CTA />
      </main>
      <Footer />
    </div>
  );
}
