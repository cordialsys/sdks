/**
 * CLI test command - run CSL test files.
 *
 * Conventions:
 * - Files prefixed with `_` are setup files, run before tests.
 * - All other `.csl` files are test files.
 * - Reports pass/fail with timing.
 */
import * as fs from "node:fs";
import * as path from "node:path";
import { CslVm, CslError } from "@cordialsys/treasury-csl";
import type { TreasuryClient } from "@cordialsys/treasury-sdk";
import { Keyring } from "@cordialsys/treasury-sdk";

export interface TestResult {
  file: string;
  passed: boolean;
  duration: number; // ms
  error?: string;
  output: string[];
}

export async function runTests(
  client: TreasuryClient,
  testPath: string,
): Promise<{ results: TestResult[]; passed: number; failed: number }> {
  const stat = fs.statSync(testPath);
  let files: string[];

  if (stat.isDirectory()) {
    files = fs
      .readdirSync(testPath)
      .filter((f) => f.endsWith(".csl"))
      .map((f) => path.join(testPath, f))
      .sort();
  } else {
    files = [testPath];
  }

  // Separate setup files (prefixed with _) from test files
  const setupFiles = files.filter((f) => path.basename(f).startsWith("_"));
  const testFiles = files.filter((f) => !path.basename(f).startsWith("_"));

  const results: TestResult[] = [];

  // Run setup files first
  for (const file of setupFiles) {
    const result = await runSingleTest(client, file);
    results.push(result);
    if (!result.passed) {
      console.error(`Setup file failed: ${file}`);
      // Continue with other tests anyway
    }
  }

  // Run test files
  for (const file of testFiles) {
    const result = await runSingleTest(client, file);
    results.push(result);
  }

  const passed = results.filter((r) => r.passed).length;
  const failed = results.filter((r) => !r.passed).length;

  return { results, passed, failed };
}

async function runSingleTest(
  client: TreasuryClient,
  filePath: string,
): Promise<TestResult> {
  const start = Date.now();
  const keyring = new Keyring(client.treasuryId);
  const vm = new CslVm(client, keyring);
  const content = fs.readFileSync(filePath, "utf-8");

  try {
    await vm.runScript(content);
    const duration = Date.now() - start;
    return {
      file: filePath,
      passed: true,
      duration,
      output: vm.getOutput(),
    };
  } catch (e) {
    const duration = Date.now() - start;
    if (e instanceof CslError && e.code === "EXIT") {
      return {
        file: filePath,
        passed: true,
        duration,
        output: vm.getOutput(),
      };
    }
    return {
      file: filePath,
      passed: false,
      duration,
      error: e instanceof Error ? e.message : String(e),
      output: vm.getOutput(),
    };
  }
}

export function printResults(results: {
  results: TestResult[];
  passed: number;
  failed: number;
}): void {
  console.log("\n--- Test Results ---\n");

  for (const r of results.results) {
    const status = r.passed ? "PASS" : "FAIL";
    const duration = `${r.duration}ms`;
    const name = path.basename(r.file);
    console.log(`  ${status}  ${name}  (${duration})`);
    if (!r.passed && r.error) {
      console.log(`         ${r.error}`);
    }
  }

  console.log(
    `\n  ${results.passed} passed, ${results.failed} failed, ${results.results.length} total\n`,
  );
}
