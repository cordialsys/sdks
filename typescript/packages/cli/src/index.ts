#!/usr/bin/env node
/**
 * Treasury CLI - Command-line interface for Treasury operations.
 */
import { Command } from "commander";
import * as fs from "node:fs";
import * as path from "node:path";
import {
  TreasuryClient,
  Keyring,
  SigningKey,
  allResourceTypes,
  singularToPlural,
} from "@cordialsys/treasury-sdk";
import { runScript } from "./commands/script.js";
import { runTests, printResults } from "./commands/test.js";

interface GlobalOpts {
  baseUrl: string;
  treasuryId?: string;
  hostId?: string;
  key?: string;
}

const program = new Command();

program
  .name("treasury")
  .description("Treasury CLI - manage Treasury resources")
  .version("0.1.0");

// --- Global options ---

program
  .option("-b, --base-url <url>", "Treasury API base URL", "http://127.0.0.1:8777")
  .option("-t, --treasury-id <id>", "Treasury ID")
  .option("-H, --host-id <id>", "Host ID")
  .option("-k, --key <name>", "Signing key name from keyring");

// --- Helper to build a client from global options ---

function buildClient(opts: {
  baseUrl: string;
  treasuryId?: string;
  hostId?: string;
  key?: string;
}): TreasuryClient {
  const treasuryId = opts.treasuryId || process.env.TREASURY_ID || "";
  if (!treasuryId) {
    console.error("Error: --treasury-id or TREASURY_ID env var is required");
    process.exit(1);
  }

  const baseUrl = opts.baseUrl || process.env.TREASURY_BASE_URL || "http://127.0.0.1:8777";

  let signingKey: SigningKey | undefined;
  const keyName = opts.key || process.env.TREASURY_KEY;
  if (keyName) {
    const keyring = new Keyring(treasuryId);
    signingKey = keyring.getSigningKey(keyName) ?? undefined;
  }

  return new TreasuryClient({
    baseUrl,
    treasuryId,
    hostId: opts.hostId || process.env.TREASURY_HOST_ID,
    signingKey,
  });
}

// --- Script command ---

program
  .command("script")
  .description("Execute a CSL script file")
  .requiredOption("-f, --file <path>", "Path to CSL script file")
  .action(async (cmdOpts) => {
    const globalOpts = program.opts<GlobalOpts>();
    const client = buildClient(globalOpts);
    const filePath = path.resolve(cmdOpts.file);

    if (!fs.existsSync(filePath)) {
      console.error(`File not found: ${filePath}`);
      process.exit(1);
    }

    const result = await runScript(client, filePath);
    for (const line of result.output) {
      console.log(line);
    }
    if (!result.success) {
      process.exit(1);
    }
  });

// --- Test command ---

program
  .command("test")
  .description("Run CSL test files")
  .argument("<path>", "Path to test file or directory")
  .action(async (testPath: string) => {
    const globalOpts = program.opts<GlobalOpts>();
    const client = buildClient(globalOpts);
    const resolved = path.resolve(testPath);

    if (!fs.existsSync(resolved)) {
      console.error(`Path not found: ${resolved}`);
      process.exit(1);
    }

    const results = await runTests(client, resolved);
    printResults(results);
    if (results.failed > 0) {
      process.exit(1);
    }
  });

// --- Config command ---

const configCmd = program
  .command("config")
  .description("Manage CLI configuration");

configCmd
  .command("show")
  .description("Show current configuration")
  .action(() => {
    const opts = program.opts<GlobalOpts>();
    console.log(JSON.stringify({
      baseUrl: opts.baseUrl || process.env.TREASURY_BASE_URL || "http://127.0.0.1:8777",
      treasuryId: opts.treasuryId || process.env.TREASURY_ID || "(not set)",
      hostId: opts.hostId || process.env.TREASURY_HOST_ID || "(not set)",
      key: opts.key || process.env.TREASURY_KEY || "(not set)",
    }, null, 2));
  });

// --- Keyring command ---

const keyringCmd = program
  .command("keyring")
  .description("Manage signing keys");

keyringCmd
  .command("list")
  .description("List all keys in the keyring")
  .action(() => {
    const globalOpts = program.opts<GlobalOpts>();
    const treasuryId = globalOpts.treasuryId || process.env.TREASURY_ID;
    if (!treasuryId) {
      console.error("Error: --treasury-id or TREASURY_ID env var is required");
      process.exit(1);
    }
    const keyring = new Keyring(treasuryId);
    const names = keyring.listNames();
    if (names.length === 0) {
      console.log("No keys in keyring.");
    } else {
      for (const name of names) {
        console.log(name);
      }
    }
  });

keyringCmd
  .command("create <name>")
  .description("Create a new signing key")
  .option("-a, --algorithm <alg>", "Algorithm (ed25519, ecdsa-k256-sha256, ecdsa-p256-sha256)", "ed25519")
  .action((name: string, cmdOpts) => {
    const globalOpts = program.opts<GlobalOpts>();
    const treasuryId = globalOpts.treasuryId || process.env.TREASURY_ID;
    if (!treasuryId) {
      console.error("Error: --treasury-id or TREASURY_ID env var is required");
      process.exit(1);
    }
    const keyring = new Keyring(treasuryId);
    keyring.create(name, cmdOpts.algorithm);
    console.log(`Created key: ${name}`);
  });

keyringCmd
  .command("import <name>")
  .description("Import a signing key from hex")
  .requiredOption("-s, --secret <hex>", "Secret key in hex")
  .option("-a, --algorithm <alg>", "Algorithm", "ed25519")
  .action((name: string, cmdOpts) => {
    const globalOpts = program.opts<GlobalOpts>();
    const treasuryId = globalOpts.treasuryId || process.env.TREASURY_ID;
    if (!treasuryId) {
      console.error("Error: --treasury-id or TREASURY_ID env var is required");
      process.exit(1);
    }
    const keyring = new Keyring(treasuryId);
    keyring.import(name, cmdOpts.algorithm, cmdOpts.secret);
    console.log(`Imported key: ${name}`);
  });

keyringCmd
  .command("delete <name>")
  .description("Delete a signing key")
  .action((name: string) => {
    const globalOpts = program.opts<GlobalOpts>();
    const treasuryId = globalOpts.treasuryId || process.env.TREASURY_ID;
    if (!treasuryId) {
      console.error("Error: --treasury-id or TREASURY_ID env var is required");
      process.exit(1);
    }
    const keyring = new Keyring(treasuryId);
    keyring.delete(name);
    console.log(`Deleted key: ${name}`);
  });

keyringCmd
  .command("show <name>")
  .description("Show a signing key's public key")
  .action((name: string) => {
    const globalOpts = program.opts<GlobalOpts>();
    const treasuryId = globalOpts.treasuryId || process.env.TREASURY_ID;
    if (!treasuryId) {
      console.error("Error: --treasury-id or TREASURY_ID env var is required");
      process.exit(1);
    }
    const keyring = new Keyring(treasuryId);
    const key = keyring.getSigningKey(name);
    if (!key) {
      console.error(`Key not found: ${name}`);
      process.exit(1);
    }
    console.log(JSON.stringify({
      name,
      algorithm: key.algorithm,
      publicKey: key.verifyingKey().toPublicBytesHex(),
    }, null, 2));
  });

// --- Generic resource commands ---

const RESOURCE_COMMANDS = [
  "access-rule", "account", "address", "airdrop", "asset", "backup",
  "call", "call-rule", "chain", "credential", "feature", "host",
  "key", "operation", "role", "signatory", "signature", "signer",
  "software-update", "staking", "staking-rule", "symbol", "tag",
  "transaction", "transfer", "transfer-rule", "treasury", "type", "user",
];

for (const resourceType of RESOURCE_COMMANDS) {
  const resCmd = program
    .command(resourceType)
    .description(`Manage ${resourceType} resources`);

  // get <name>
  resCmd
    .command("get <name>")
    .description(`Get a ${resourceType}`)
    .action(async (name: string) => {
      const globalOpts = program.opts<GlobalOpts>();
      const client = buildClient(globalOpts);
      const resourceName = name.includes("/") ? name : `${singularToPlural(resourceType)}/${name}`;
      try {
        const result = await client.get(resourceName);
        console.log(JSON.stringify(result, null, 2));
      } catch (e) {
        console.error(e instanceof Error ? e.message : String(e));
        process.exit(1);
      }
    });

  // list
  resCmd
    .command("list")
    .description(`List ${resourceType} resources`)
    .option("-p, --parent <id>", "Parent resource ID (for nested resources)")
    .option("--filter <expr>", "Filter expression")
    .option("--order-by <field>", "Order by field")
    .option("--page-size <n>", "Page size", parseInt)
    .option("--page-token <token>", "Page token")
    .option("--show-deleted", "Show deleted resources")
    .action(async (cmdOpts) => {
      const globalOpts = program.opts<GlobalOpts>();
      const client = buildClient(globalOpts);
      try {
        const result = await client.list(resourceType, {
          parentId: cmdOpts.parent,
          filter: cmdOpts.filter,
          orderBy: cmdOpts.orderBy,
          pageSize: cmdOpts.pageSize,
          pageToken: cmdOpts.pageToken,
          deleted: cmdOpts.showDeleted,
        });
        console.log(JSON.stringify(result, null, 2));
      } catch (e) {
        console.error(e instanceof Error ? e.message : String(e));
        process.exit(1);
      }
    });

  // create [name]
  resCmd
    .command("create [name]")
    .description(`Create a ${resourceType}`)
    .option("-p, --parent <id>", "Parent resource ID (for nested resources)")
    .option("-d, --data <json>", "JSON data for the resource", "{}")
    .option("-f, --file <path>", "Read JSON data from file")
    .option("-w, --wait", "Wait for operation to complete")
    .action(async (name: string | undefined, cmdOpts) => {
      const globalOpts = program.opts<GlobalOpts>();
      const client = buildClient(globalOpts);
      let data: Record<string, unknown>;
      if (cmdOpts.file) {
        data = JSON.parse(fs.readFileSync(cmdOpts.file, "utf-8"));
      } else {
        data = JSON.parse(cmdOpts.data);
      }
      try {
        if (cmdOpts.wait) {
          const result = await client.createAndWait(resourceType, data, {
            id: name,
            parentId: cmdOpts.parent,
          });
          console.log(JSON.stringify(result, null, 2));
        } else {
          const opName = await client.create(resourceType, data, {
            id: name,
            parentId: cmdOpts.parent,
          });
          console.log(opName);
        }
      } catch (e) {
        console.error(e instanceof Error ? e.message : String(e));
        process.exit(1);
      }
    });

  // update <name>
  resCmd
    .command("update <name>")
    .description(`Update a ${resourceType}`)
    .option("-d, --data <json>", "JSON data for the update", "{}")
    .option("-f, --file <path>", "Read JSON data from file")
    .action(async (name: string, cmdOpts) => {
      const globalOpts = program.opts<GlobalOpts>();
      const client = buildClient(globalOpts);
      const resourceName = name.includes("/") ? name : `${singularToPlural(resourceType)}/${name}`;
      let data: Record<string, unknown>;
      if (cmdOpts.file) {
        data = JSON.parse(fs.readFileSync(cmdOpts.file, "utf-8"));
      } else {
        data = JSON.parse(cmdOpts.data);
      }
      try {
        const opName = await client.update(resourceName, data);
        console.log(opName);
      } catch (e) {
        console.error(e instanceof Error ? e.message : String(e));
        process.exit(1);
      }
    });

  // delete <name>
  resCmd
    .command("delete <name>")
    .description(`Delete a ${resourceType}`)
    .action(async (name: string) => {
      const globalOpts = program.opts<GlobalOpts>();
      const client = buildClient(globalOpts);
      const resourceName = name.includes("/") ? name : `${singularToPlural(resourceType)}/${name}`;
      try {
        const opName = await client.delete(resourceName);
        console.log(opName);
      } catch (e) {
        console.error(e instanceof Error ? e.message : String(e));
        process.exit(1);
      }
    });

  // approve <operation-name>
  if (resourceType === "operation") {
    resCmd
      .command("approve <name>")
      .description("Approve an operation")
      .action(async (name: string) => {
        const globalOpts = program.opts<GlobalOpts>();
        const client = buildClient(globalOpts);
        const opName = name.includes("/") ? name : `operations/${name}`;
        try {
          const result = await client.approve(opName);
          console.log(result);
        } catch (e) {
          console.error(e instanceof Error ? e.message : String(e));
          process.exit(1);
        }
      });

    resCmd
      .command("cancel <name>")
      .description("Cancel an operation")
      .action(async (name: string) => {
        const globalOpts = program.opts<GlobalOpts>();
        const client = buildClient(globalOpts);
        const opName = name.includes("/") ? name : `operations/${name}`;
        try {
          const result = await client.cancel(opName);
          console.log(result);
        } catch (e) {
          console.error(e instanceof Error ? e.message : String(e));
          process.exit(1);
        }
      });

    resCmd
      .command("wait <name>")
      .description("Wait for an operation to complete")
      .action(async (name: string) => {
        const globalOpts = program.opts<GlobalOpts>();
        const client = buildClient(globalOpts);
        const opName = name.includes("/") ? name : `operations/${name}`;
        try {
          const result = await client.waitForOperation(opName);
          console.log(JSON.stringify(result, null, 2));
        } catch (e) {
          console.error(e instanceof Error ? e.message : String(e));
          process.exit(1);
        }
      });
  }
}

program.parseAsync(process.argv).catch((e) => {
  console.error(e instanceof Error ? e.message : String(e));
  process.exit(1);
});
