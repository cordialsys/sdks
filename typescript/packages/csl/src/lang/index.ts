/**
 * CSL Abstract Syntax Tree types.
 */

export type InlineTable = Record<string, unknown>;

export interface Variable {
  identifier: string;
  path: string[];
}

export type DelayedFunction =
  | "keccak256"
  | "sha256"
  | "resource"
  | "typed-data"
  | "verify-signature"
  | "parse-call"
  | "json";

export interface FunctionCall {
  name: DelayedFunction;
  parameters: Value[];
}

export type Value =
  | { type: "string"; value: string }
  | { type: "integer"; value: number }
  | { type: "float"; value: number }
  | { type: "boolean"; value: boolean }
  | { type: "array"; value: Value[] }
  | { type: "inline-table"; value: InlineTable }
  | { type: "variable"; value: Variable }
  | { type: "function"; value: FunctionCall };

export interface Partial {
  resource: string; // singular resource type name
  id?: string;
  parentId?: string;
}

export type PartialOrVariable =
  | { type: "name"; value: Partial }
  | { type: "variable"; value: string };

export interface IdOrVariable {
  type: "id" | "variable";
  value: string;
}

// Commands
export interface Get {
  pov: PartialOrVariable;
  proposed: boolean;
}

export interface List {
  resource: string;
  parent?: IdOrVariable;
  filter?: string;
  proposed: boolean;
}

export type Read = { type: "get"; value: Get } | { type: "list"; value: List };

export interface Create {
  variant?: string;
  partial: Partial;
  data: InlineTable;
}

export interface Update {
  what: PartialOrVariable;
  data: InlineTable;
}

export interface Delete {
  pov: PartialOrVariable;
  proposed: boolean;
}

export type CustomPayload =
  | { type: "string"; value: string }
  | { type: "object"; value: InlineTable };

export interface Custom {
  action: string;
  what: PartialOrVariable;
  payload?: CustomPayload;
}

export interface Assert {
  condition: string;
}

export interface Convert {
  data: Value;
  sourceEncoding: Value;
  targetEncoding: Value;
}

export interface Replace {
  data: Value;
  old: Value;
  new: Value;
}

export interface ForLoop {
  variable: string;
  iterable: { type: "array"; value: Value[] } | { type: "list"; value: List };
  command: Command;
  bodyText?: string; // Raw body text for multi-command bodies
}

export interface Until {
  condition: string;
}

export interface Setting {
  key: string[];
  value: string;
  value2?: string;
  value3?: string;
}

export interface SettingKey {
  key: string[];
}

export type Assignable =
  | { type: "create"; value: Create }
  | { type: "propose"; value: Create }
  | { type: "update"; value: Update }
  | { type: "delete"; value: Delete }
  | { type: "approve"; value: PartialOrVariable }
  | { type: "submit"; value: PartialOrVariable }
  | { type: "cancel"; value: PartialOrVariable }
  | { type: "get"; value: Get }
  | { type: "partial"; value: Partial }
  | { type: "string"; value: Value }
  | { type: "setting"; value: SettingKey }
  | { type: "function"; value: FunctionCall }
  | { type: "custom"; value: Custom }
  | { type: "convert"; value: Convert }
  | { type: "replace"; value: Replace };

export type Mutate =
  | { type: "create"; value: Create }
  | { type: "propose"; value: Create }
  | { type: "get"; value: Get }
  | { type: "update"; value: Update }
  | { type: "delete"; value: Delete }
  | { type: "approve"; value: PartialOrVariable }
  | { type: "submit"; value: PartialOrVariable }
  | { type: "cancel"; value: PartialOrVariable }
  | { type: "custom"; value: Custom };

export interface Assignment {
  variable: string;
  value: Assignable;
  direct: boolean;
}

export interface FallibleAssignment {
  variable: string;
  error: string;
  mutate: Mutate;
  direct: boolean;
}

export type Command =
  | { type: "nop" }
  | { type: "exit" }
  | { type: "read"; value: Read }
  | { type: "assert"; value: Assert }
  | { type: "convert"; value: Convert }
  | { type: "replace"; value: Replace }
  | { type: "assignment"; value: Assignment }
  | { type: "fallible-assignment"; value: FallibleAssignment }
  | { type: "create"; value: Create }
  | { type: "propose"; value: Create }
  | { type: "for-loop"; value: ForLoop }
  | { type: "until"; value: Until }
  | { type: "update"; value: Update }
  | { type: "delete"; value: Delete }
  | { type: "custom"; value: Custom }
  | { type: "set-setting"; value: Setting }
  | { type: "get-setting"; value: SettingKey }
  | { type: "unset"; value: SettingKey }
  | { type: "approve"; value: PartialOrVariable }
  | { type: "submit"; value: PartialOrVariable }
  | { type: "cancel"; value: PartialOrVariable }
  | { type: "value"; value: Value };
