// Runs tests/Verify.elm against testdata/ai-trace.json under Node.
//
//   node tests/verify.mjs
//
// Compiles the worker first, so there is nothing to install beyond what
// build.sh already needs.
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { createRequire } from "node:module";
import { join, resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, "..");
const tracePath = resolve(root, "../../testdata/ai-trace.json");

const scratch = mkdtempSync(join(tmpdir(), "elm-verify-"));
const bundle = join(scratch, "verify.js");

try {
  // Windows needs a shell to run npx.cmd, which in turn needs the path quoted.
  const shell = process.platform === "win32";
  const output = shell ? `--output="${bundle}"` : `--output=${bundle}`;
  execFileSync(
    shell ? "npx.cmd" : "npx",
    ["--yes", "elm@0.19.2-0", "make", "tests/Verify.elm", output],
    { cwd: root, stdio: ["ignore", "ignore", "inherit"], shell },
  );

  // The compiler emits a CommonJS-friendly bundle, so load it that way.
  const { Elm } = createRequire(import.meta.url)(bundle);

  const samples = JSON.parse(readFileSync(tracePath, "utf8"));
  const app = Elm.Verify.init({ flags: samples });

  const result = await new Promise((done) => app.ports.report.subscribe(done));
  const [tier1, tier2, fallback] = result.tiers;

  console.log(
    `checked ${result.checked} samples: tier1 ${tier1}, tier2 ${tier2}, fallback ${fallback}`,
  );
  if (result.problems.length > 0) {
    for (const problem of result.problems.slice(0, 10)) console.log("  " + problem);
    console.log(`${result.problems.length} mismatches`);
    process.exit(1);
  }
  if (result.checked === 0 || tier1 === 0 || tier2 === 0 || fallback === 0) {
    console.log("trace did not exercise all three tiers");
    process.exit(1);
  }
  console.log("Elm demo AI matches the reference.");
} finally {
  rmSync(scratch, { recursive: true, force: true });
}
