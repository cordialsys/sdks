import { describe, it } from "node:test";
import * as assert from "node:assert/strict";
import { parseCommand, splitCommands, EmptyVarStore } from "./index.js";

const vars = new EmptyVarStore();

describe("splitCommands", () => {
  it("splits by semicolons", () => {
    const result = splitCommands("a = 1; b = 2");
    assert.deepEqual(result, ["a = 1", "b = 2"]);
  });

  it("splits by newlines", () => {
    const result = splitCommands("a = 1\nb = 2");
    assert.deepEqual(result, ["a = 1", "b = 2"]);
  });

  it("skips comments", () => {
    const result = splitCommands("a = 1\n# comment\nb = 2");
    assert.deepEqual(result, ["a = 1", "b = 2"]);
  });

  it("skips shebangs", () => {
    const result = splitCommands("#!/usr/bin/env treasury\na = 1");
    assert.deepEqual(result, ["a = 1"]);
  });

  it("keeps blocks together", () => {
    const result = splitCommands("for x in items {\na = $x\nb = $x\n}");
    assert.equal(result.length, 1);
    assert.ok(result[0].includes("for x in items"));
  });

  it("handles empty input", () => {
    const result = splitCommands("");
    assert.deepEqual(result, []);
  });
});

describe("parseCommand", () => {
  it("parses exit", () => {
    const cmd = parseCommand("exit", vars);
    assert.equal(cmd.type, "exit");
  });

  it("parses quit", () => {
    const cmd = parseCommand("quit", vars);
    assert.equal(cmd.type, "exit");
  });

  it("parses set setting", () => {
    const cmd = parseCommand("set sign.with = root-key", vars);
    assert.equal(cmd.type, "set-setting");
    if (cmd.type === "set-setting") {
      assert.deepEqual(cmd.value.key, ["sign", "with"]);
      assert.equal(cmd.value.value, "root-key");
    }
  });

  it("parses simple assignment", () => {
    const cmd = parseCommand("x = get chain ETH", vars);
    assert.equal(cmd.type, "assignment");
    if (cmd.type === "assignment") {
      assert.equal(cmd.value.variable, "x");
    }
  });

  it("parses get", () => {
    const cmd = parseCommand("get chain ETH", vars);
    assert.equal(cmd.type, "read");
  });

  it("parses list", () => {
    const cmd = parseCommand("list chains", vars);
    assert.equal(cmd.type, "read");
  });

  it("parses create", () => {
    const cmd = parseCommand("create internal account id(test) {}", vars);
    assert.equal(cmd.type, "create");
    if (cmd.type === "create") {
      assert.equal(cmd.value.partial.resource, "account");
      assert.equal(cmd.value.variant, "internal");
      assert.equal(cmd.value.partial.id, "test");
    }
  });

  it("parses delete", () => {
    const cmd = parseCommand("delete chain MYCHAIN", vars);
    assert.equal(cmd.type, "delete");
  });

  it("parses assert with condition", () => {
    const cmd = parseCommand('assert $err.status = "Invalid Argument"', vars);
    assert.equal(cmd.type, "assert");
  });

  it("parses fallible assignment", () => {
    const cmd = parseCommand('_, err = create native chain MYNATIVECHAIN { confirmations = 6 }', vars);
    assert.equal(cmd.type, "fallible-assignment");
    if (cmd.type === "fallible-assignment") {
      assert.equal(cmd.value.variable, "_");
      assert.equal(cmd.value.error, "err");
    }
  });

  it("parses update with inline table", () => {
    const cmd = parseCommand('update $chain { fee_limits = { ETH = "2" } }', vars);
    assert.equal(cmd.type, "update");
  });

  it("parses approve", () => {
    const cmd = parseCommand("approve $op", vars);
    assert.equal(cmd.type, "approve");
  });

  it("parses cancel", () => {
    const cmd = parseCommand("cancel $op", vars);
    assert.equal(cmd.type, "cancel");
  });

  it("parses custom action", () => {
    const cmd = parseCommand('fee-payer $account "allow"', vars);
    assert.equal(cmd.type, "custom");
  });

  it("parses for loop with list", () => {
    const cmd = parseCommand("for x in list chains {\nget chain $x\n}", vars);
    assert.equal(cmd.type, "for-loop");
  });

  it("parses until", () => {
    const cmd = parseCommand('until $transfer.state = "succeeded"', vars);
    assert.equal(cmd.type, "until");
  });

  it("parses nonce function", () => {
    const cmd = parseCommand("id = nonce()", vars);
    assert.equal(cmd.type, "assignment");
    if (cmd.type === "assignment") {
      assert.equal(cmd.value.variable, "id");
    }
  });
});
