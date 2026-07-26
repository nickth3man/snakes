export interface BenchmarkResult {
  runs: number;
  seed: number;
  timestamp: string;
  version: number;
  mean: number;
  median: number;
  max: number;
  min: number;
  p90: number;
  winRate: number;
  avgSteps: number;
  scores: number[];
}
