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
  const results: TestResult[] = [];

  if (stat.isDirectory()) {
    await runDirectory(client, testPath, results);
  } else {
    if (!testPath.endsWith(".csl")) {
      throw new Error(`Not a CSL test file: ${testPath}`);
    }
    const result = await runSingleTest(client, testPath);
    results.push(result);
  }

  const passed = results.filter((r) => r.passed).length;
  const failed = results.filter((r) => !r.passed).length;

  return { results, passed, failed };
}

async function runDirectory(
  client: TreasuryClient,
  dir: string,
  results: TestResult[],
): Promise<void> {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  const files = entries
    .filter((entry) => entry.isFile() && entry.name.endsWith(".csl"))
    .map((entry) => path.join(dir, entry.name))
    .sort();

  const setupFiles = files.filter((file) => path.basename(file).startsWith("_"));
  const testFiles = files.filter((file) => !path.basename(file).startsWith("_"));

  for (const file of [...setupFiles, ...testFiles]) {
    const result = await runSingleTest(client, file);
    results.push(result);
    if (!result.passed && path.basename(file).startsWith("_")) {
      console.error(`Setup file failed: ${file}`);
    }
  }

  const subdirs = entries
    .filter((entry) => entry.isDirectory())
    .map((entry) => path.join(dir, entry.name))
    .sort();

  for (const subdir of subdirs) {
    await runDirectory(client, subdir, results);
  }
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
