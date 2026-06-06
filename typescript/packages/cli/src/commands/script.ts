/**
 * CLI script command - execute a CSL script file.
 */
import * as fs from "node:fs";
import { CslVm, CslError } from "@cordialsys/treasury-csl";
import type { TreasuryClient } from "@cordialsys/treasury-sdk";
import { Keyring } from "@cordialsys/treasury-sdk";

export async function runScript(
  client: TreasuryClient,
  filePath: string,
): Promise<{ success: boolean; output: string[] }> {
  const keyring = new Keyring(client.treasuryId);
  const vm = new CslVm(client, keyring);

  const content = fs.readFileSync(filePath, "utf-8");

  try {
    await vm.runScript(content);
    return { success: true, output: vm.getOutput() };
  } catch (e) {
    if (e instanceof CslError && e.code === "EXIT") {
      return { success: true, output: vm.getOutput() };
    }
    const output = vm.getOutput();
    if (e instanceof Error) {
      output.push(`ERROR: ${e.message}`);
    }
    return { success: false, output };
  }
}
