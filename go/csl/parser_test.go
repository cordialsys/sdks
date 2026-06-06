package csl

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustParseLine(t *testing.T, line string) Command {
	t.Helper()
	cmd, err := ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine(%q): unexpected error: %v", line, err)
	}
	return cmd
}

func mustParse(t *testing.T, source string) *Program {
	t.Helper()
	prog, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse: unexpected error: %v", err)
	}
	return prog
}

// ---------------------------------------------------------------------------
// Nop (comments, empty lines, shebang)
// ---------------------------------------------------------------------------

func TestNop_EmptyLine(t *testing.T) {
	cmd := mustParseLine(t, "")
	if _, ok := cmd.(Nop); !ok {
		t.Fatalf("expected Nop, got %T", cmd)
	}
}

func TestNop_Whitespace(t *testing.T) {
	cmd := mustParseLine(t, "   ")
	if _, ok := cmd.(Nop); !ok {
		t.Fatalf("expected Nop, got %T", cmd)
	}
}

func TestNop_HashComment(t *testing.T) {
	cmd := mustParseLine(t, "# this is a comment")
	if _, ok := cmd.(Nop); !ok {
		t.Fatalf("expected Nop, got %T", cmd)
	}
}

func TestNop_SlashComment(t *testing.T) {
	cmd := mustParseLine(t, "// this is a comment")
	if _, ok := cmd.(Nop); !ok {
		t.Fatalf("expected Nop, got %T", cmd)
	}
}

func TestNop_Shebang(t *testing.T) {
	cmd := mustParseLine(t, "#!/usr/bin/env csl")
	if _, ok := cmd.(Nop); !ok {
		t.Fatalf("expected Nop, got %T", cmd)
	}
}

// ---------------------------------------------------------------------------
// Exit / ExitIfSet
// ---------------------------------------------------------------------------

func TestExit(t *testing.T) {
	cmd := mustParseLine(t, "exit")
	if _, ok := cmd.(Exit); !ok {
		t.Fatalf("expected Exit, got %T", cmd)
	}
}

func TestExitIfSet(t *testing.T) {
	cmd := mustParseLine(t, "exit_if_set MY_VAR")
	eis, ok := cmd.(ExitIfSet)
	if !ok {
		t.Fatalf("expected ExitIfSet, got %T", cmd)
	}
	if eis.EnvVar != "MY_VAR" {
		t.Fatalf("expected MY_VAR, got %s", eis.EnvVar)
	}
}

// ---------------------------------------------------------------------------
// SetSetting / UnsetSetting / GetSetting
// ---------------------------------------------------------------------------

func TestSetSetting(t *testing.T) {
	cmd := mustParseLine(t, `set api url "https://api.example.com"`)
	ss, ok := cmd.(SetSetting)
	if !ok {
		t.Fatalf("expected SetSetting, got %T", cmd)
	}
	if len(ss.Key) != 2 || ss.Key[0] != "api" || ss.Key[1] != "url" {
		t.Fatalf("unexpected key: %v", ss.Key)
	}
	if ss.Value != `"https://api.example.com"` {
		t.Fatalf("unexpected value: %s", ss.Value)
	}
}

func TestUnsetSetting(t *testing.T) {
	cmd := mustParseLine(t, "unset api url")
	us, ok := cmd.(UnsetSetting)
	if !ok {
		t.Fatalf("expected UnsetSetting, got %T", cmd)
	}
	if len(us.Key) != 2 || us.Key[0] != "api" || us.Key[1] != "url" {
		t.Fatalf("unexpected key: %v", us.Key)
	}
}

func TestGetSetting(t *testing.T) {
	cmd := mustParseLine(t, "get setting api url")
	gs, ok := cmd.(GetSetting)
	if !ok {
		t.Fatalf("expected GetSetting, got %T", cmd)
	}
	if len(gs.Key) != 2 || gs.Key[0] != "api" || gs.Key[1] != "url" {
		t.Fatalf("unexpected key: %v", gs.Key)
	}
}

// ---------------------------------------------------------------------------
// Assert / Until
// ---------------------------------------------------------------------------

func TestAssert(t *testing.T) {
	cmd := mustParseLine(t, `assert $status == "active"`)
	a, ok := cmd.(Assert)
	if !ok {
		t.Fatalf("expected Assert, got %T", cmd)
	}
	if !strings.Contains(a.Condition, `$status == "active"`) {
		t.Fatalf("unexpected condition: %s", a.Condition)
	}
}

func TestUntil(t *testing.T) {
	cmd := mustParseLine(t, `until $status == "completed"`)
	u, ok := cmd.(Until)
	if !ok {
		t.Fatalf("expected Until, got %T", cmd)
	}
	if !strings.Contains(u.Condition, `$status == "completed"`) {
		t.Fatalf("unexpected condition: %s", u.Condition)
	}
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestCreate_SimpleResource(t *testing.T) {
	cmd := mustParseLine(t, "create account")
	c, ok := cmd.(Create)
	if !ok {
		t.Fatalf("expected Create, got %T", cmd)
	}
	if c.ResourceType != ResourceAccount {
		t.Fatalf("expected Account, got %s", c.ResourceType)
	}
	if c.Variant != nil {
		t.Fatalf("expected nil variant, got %s", *c.Variant)
	}
}

func TestCreate_WithVariant(t *testing.T) {
	cmd := mustParseLine(t, "create internal account")
	c, ok := cmd.(Create)
	if !ok {
		t.Fatalf("expected Create, got %T", cmd)
	}
	if c.ResourceType != ResourceAccount {
		t.Fatalf("expected Account, got %s", c.ResourceType)
	}
	if c.Variant == nil || *c.Variant != "internal" {
		t.Fatalf("expected variant 'internal', got %v", c.Variant)
	}
}

func TestCreate_WithId(t *testing.T) {
	cmd := mustParseLine(t, "create user my-user")
	c, ok := cmd.(Create)
	if !ok {
		t.Fatalf("expected Create, got %T", cmd)
	}
	if c.ResourceType != ResourceUser {
		t.Fatalf("expected User, got %s", c.ResourceType)
	}
	if c.Id == nil || *c.Id != "my-user" {
		t.Fatalf("expected id 'my-user', got %v", c.Id)
	}
}

func TestCreate_WithParent(t *testing.T) {
	cmd := mustParseLine(t, "create address for ETH")
	c, ok := cmd.(Create)
	if !ok {
		t.Fatalf("expected Create, got %T", cmd)
	}
	if c.ResourceType != ResourceAddress {
		t.Fatalf("expected Address, got %s", c.ResourceType)
	}
	if c.Parent == nil || *c.Parent != "ETH" {
		t.Fatalf("expected parent 'ETH', got %v", c.Parent)
	}
}

func TestCreate_WithExtension(t *testing.T) {
	cmd := mustParseLine(t, "create transfer with memo1")
	c, ok := cmd.(Create)
	if !ok {
		t.Fatalf("expected Create, got %T", cmd)
	}
	if c.Extension == nil || *c.Extension != "memo1" {
		t.Fatalf("expected extension 'memo1', got %v", c.Extension)
	}
}

func TestCreate_WithData(t *testing.T) {
	cmd := mustParseLine(t, `create account { name = "test", active = true }`)
	c, ok := cmd.(Create)
	if !ok {
		t.Fatalf("expected Create, got %T", cmd)
	}
	if c.Data == nil {
		t.Fatalf("expected data, got nil")
	}
	if len(c.Data) != 2 {
		t.Fatalf("expected 2 data entries, got %d", len(c.Data))
	}
}

func TestCreate_WithVariantAndParentAndData(t *testing.T) {
	cmd := mustParseLine(t, `create internal address for SOL { label = "primary" }`)
	c, ok := cmd.(Create)
	if !ok {
		t.Fatalf("expected Create, got %T", cmd)
	}
	if c.ResourceType != ResourceAddress {
		t.Fatalf("expected Address, got %s", c.ResourceType)
	}
	if c.Variant == nil || *c.Variant != "internal" {
		t.Fatalf("expected variant 'internal'")
	}
	if c.Parent == nil || *c.Parent != "SOL" {
		t.Fatalf("expected parent 'SOL'")
	}
	if c.Data == nil || len(c.Data) != 1 {
		t.Fatalf("expected 1 data entry")
	}
}

func TestCreate_HyphenatedResource(t *testing.T) {
	cmd := mustParseLine(t, "create allow access-rule")
	c, ok := cmd.(Create)
	if !ok {
		t.Fatalf("expected Create, got %T", cmd)
	}
	if c.ResourceType != ResourceAccessRule {
		t.Fatalf("expected AccessRule, got %s", c.ResourceType)
	}
	if c.Variant == nil || *c.Variant != "allow" {
		t.Fatalf("expected variant 'allow'")
	}
}

func TestCreate_CredentialVariant(t *testing.T) {
	cmd := mustParseLine(t, "create ed255 credential")
	c, ok := cmd.(Create)
	if !ok {
		t.Fatalf("expected Create, got %T", cmd)
	}
	if c.ResourceType != ResourceCredential {
		t.Fatalf("expected Credential, got %s", c.ResourceType)
	}
	if c.Variant == nil || *c.Variant != "ed255" {
		t.Fatalf("expected variant 'ed255'")
	}
}

func TestCreate_IdFunction(t *testing.T) {
	cmd := mustParseLine(t, `create user id("alice")`)
	c, ok := cmd.(Create)
	if !ok {
		t.Fatalf("expected Create, got %T", cmd)
	}
	if c.Id == nil || *c.Id != "alice" {
		t.Fatalf("expected id 'alice', got %v", c.Id)
	}
}

// ---------------------------------------------------------------------------
// Propose
// ---------------------------------------------------------------------------

func TestPropose(t *testing.T) {
	cmd := mustParseLine(t, "propose transfer for ETH")
	pr, ok := cmd.(Propose)
	if !ok {
		t.Fatalf("expected Propose, got %T", cmd)
	}
	if pr.ResourceType != ResourceTransfer {
		t.Fatalf("expected Transfer, got %s", pr.ResourceType)
	}
	if pr.Parent == nil || *pr.Parent != "ETH" {
		t.Fatalf("expected parent 'ETH'")
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestUpdate_Variable(t *testing.T) {
	cmd := mustParseLine(t, `update $myaccount { name = "new-name" }`)
	u, ok := cmd.(Update)
	if !ok {
		t.Fatalf("expected Update, got %T", cmd)
	}
	if !u.Target.IsVariable {
		t.Fatalf("expected variable target")
	}
	if u.Target.VarName != "myaccount" {
		t.Fatalf("expected varname 'myaccount', got %s", u.Target.VarName)
	}
	if u.Data == nil || len(u.Data) != 1 {
		t.Fatalf("expected 1 data entry")
	}
}

func TestUpdate_ResourceWithId(t *testing.T) {
	cmd := mustParseLine(t, `update account my-account { status = "active" }`)
	u, ok := cmd.(Update)
	if !ok {
		t.Fatalf("expected Update, got %T", cmd)
	}
	if u.Target.IsVariable {
		t.Fatalf("expected non-variable target")
	}
	if u.Target.ResourceType != ResourceAccount {
		t.Fatalf("expected Account")
	}
	if u.Target.Id != "my-account" {
		t.Fatalf("expected id 'my-account', got %s", u.Target.Id)
	}
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestDelete_Variable(t *testing.T) {
	cmd := mustParseLine(t, "delete $mytransfer")
	d, ok := cmd.(Delete)
	if !ok {
		t.Fatalf("expected Delete, got %T", cmd)
	}
	if !d.Target.IsVariable {
		t.Fatalf("expected variable target")
	}
	if d.Target.VarName != "mytransfer" {
		t.Fatalf("expected varname 'mytransfer'")
	}
}

func TestDelete_Proposed(t *testing.T) {
	cmd := mustParseLine(t, "delete proposed transfer my-transfer")
	d, ok := cmd.(Delete)
	if !ok {
		t.Fatalf("expected Delete, got %T", cmd)
	}
	if !d.Proposed {
		t.Fatalf("expected Proposed=true")
	}
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

func TestGet_Variable(t *testing.T) {
	cmd := mustParseLine(t, "get $myresource")
	g, ok := cmd.(Get)
	if !ok {
		t.Fatalf("expected Get, got %T", cmd)
	}
	if !g.Target.IsVariable {
		t.Fatalf("expected variable target")
	}
}

func TestGet_Resource(t *testing.T) {
	cmd := mustParseLine(t, "get account my-account")
	g, ok := cmd.(Get)
	if !ok {
		t.Fatalf("expected Get, got %T", cmd)
	}
	if g.Target.ResourceType != ResourceAccount {
		t.Fatalf("expected Account")
	}
	if g.Target.Id != "my-account" {
		t.Fatalf("expected id 'my-account'")
	}
}

func TestGet_Proposed(t *testing.T) {
	cmd := mustParseLine(t, "get proposed transfer my-xfer")
	g, ok := cmd.(Get)
	if !ok {
		t.Fatalf("expected Get, got %T", cmd)
	}
	if !g.Proposed {
		t.Fatalf("expected Proposed=true")
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestList_Plural(t *testing.T) {
	cmd := mustParseLine(t, "list accounts")
	l, ok := cmd.(ListCmd)
	if !ok {
		t.Fatalf("expected ListCmd, got %T", cmd)
	}
	if l.ResourceType != ResourceAccount {
		t.Fatalf("expected Account, got %s", l.ResourceType)
	}
}

func TestList_TwoWordPlural(t *testing.T) {
	cmd := mustParseLine(t, "list access rules")
	l, ok := cmd.(ListCmd)
	if !ok {
		t.Fatalf("expected ListCmd, got %T", cmd)
	}
	if l.ResourceType != ResourceAccessRule {
		t.Fatalf("expected AccessRule, got %s", l.ResourceType)
	}
}

func TestList_WithParent(t *testing.T) {
	cmd := mustParseLine(t, "list addresses for ETH")
	l, ok := cmd.(ListCmd)
	if !ok {
		t.Fatalf("expected ListCmd, got %T", cmd)
	}
	if l.ResourceType != ResourceAddress {
		t.Fatalf("expected Address")
	}
	if l.Parent == nil || l.Parent.Id != "ETH" {
		t.Fatalf("expected parent 'ETH'")
	}
}

func TestList_WithVariableParent(t *testing.T) {
	cmd := mustParseLine(t, "list addresses for $chain")
	l, ok := cmd.(ListCmd)
	if !ok {
		t.Fatalf("expected ListCmd, got %T", cmd)
	}
	if l.Parent == nil || !l.Parent.IsVariable || l.Parent.VarName != "chain" {
		t.Fatalf("expected variable parent '$chain'")
	}
}

func TestList_WithFilter(t *testing.T) {
	cmd := mustParseLine(t, `list accounts | name == "test"`)
	l, ok := cmd.(ListCmd)
	if !ok {
		t.Fatalf("expected ListCmd, got %T", cmd)
	}
	if l.Filter == nil {
		t.Fatalf("expected filter")
	}
	if !strings.Contains(*l.Filter, `name == "test"`) {
		t.Fatalf("unexpected filter: %s", *l.Filter)
	}
}

func TestList_Proposed(t *testing.T) {
	cmd := mustParseLine(t, "list proposed transfers")
	l, ok := cmd.(ListCmd)
	if !ok {
		t.Fatalf("expected ListCmd, got %T", cmd)
	}
	if !l.Proposed {
		t.Fatalf("expected Proposed=true")
	}
}

// ---------------------------------------------------------------------------
// Assignment
// ---------------------------------------------------------------------------

func TestAssignment_ValueLiteral(t *testing.T) {
	cmd := mustParseLine(t, `$name = "hello"`)
	a, ok := cmd.(Assignment)
	if !ok {
		t.Fatalf("expected Assignment, got %T", cmd)
	}
	if a.Variable != "name" {
		t.Fatalf("expected variable 'name', got %s", a.Variable)
	}
	if a.Direct {
		t.Fatalf("expected Direct=false")
	}
	va, ok := a.Value.(ValueAssignable)
	if !ok {
		t.Fatalf("expected ValueAssignable, got %T", a.Value)
	}
	sv, ok := va.Value.(StringValue)
	if !ok {
		t.Fatalf("expected StringValue, got %T", va.Value)
	}
	if sv.Value != "hello" {
		t.Fatalf("expected 'hello', got '%s'", sv.Value)
	}
}

func TestAssignment_Direct(t *testing.T) {
	cmd := mustParseLine(t, "$x := 42")
	a, ok := cmd.(Assignment)
	if !ok {
		t.Fatalf("expected Assignment, got %T", cmd)
	}
	if !a.Direct {
		t.Fatalf("expected Direct=true")
	}
	va, ok := a.Value.(ValueAssignable)
	if !ok {
		t.Fatalf("expected ValueAssignable, got %T", a.Value)
	}
	iv, ok := va.Value.(IntegerValue)
	if !ok {
		t.Fatalf("expected IntegerValue, got %T", va.Value)
	}
	if iv.Value != 42 {
		t.Fatalf("expected 42, got %d", iv.Value)
	}
}

func TestAssignment_Command(t *testing.T) {
	cmd := mustParseLine(t, "$acct = create account")
	a, ok := cmd.(Assignment)
	if !ok {
		t.Fatalf("expected Assignment, got %T", cmd)
	}
	ca, ok := a.Value.(CommandAssignable)
	if !ok {
		t.Fatalf("expected CommandAssignable, got %T", a.Value)
	}
	if _, ok := ca.Cmd.(Create); !ok {
		t.Fatalf("expected Create command, got %T", ca.Cmd)
	}
}

func TestAssignment_GetCommand(t *testing.T) {
	cmd := mustParseLine(t, "$acct = get account my-account")
	a, ok := cmd.(Assignment)
	if !ok {
		t.Fatalf("expected Assignment, got %T", cmd)
	}
	ca, ok := a.Value.(CommandAssignable)
	if !ok {
		t.Fatalf("expected CommandAssignable, got %T", a.Value)
	}
	if _, ok := ca.Cmd.(Get); !ok {
		t.Fatalf("expected Get command, got %T", ca.Cmd)
	}
}

func TestAssignment_ListCommand(t *testing.T) {
	cmd := mustParseLine(t, "$accts = list accounts")
	a, ok := cmd.(Assignment)
	if !ok {
		t.Fatalf("expected Assignment, got %T", cmd)
	}
	ca, ok := a.Value.(CommandAssignable)
	if !ok {
		t.Fatalf("expected CommandAssignable, got %T", a.Value)
	}
	if _, ok := ca.Cmd.(ListCmd); !ok {
		t.Fatalf("expected ListCmd, got %T", ca.Cmd)
	}
}

// ---------------------------------------------------------------------------
// FallibleAssignment
// ---------------------------------------------------------------------------

func TestFallibleAssignment(t *testing.T) {
	cmd := mustParseLine(t, "$result, $err = get account my-account")
	fa, ok := cmd.(FallibleAssignment)
	if !ok {
		t.Fatalf("expected FallibleAssignment, got %T", cmd)
	}
	if fa.Variable != "result" {
		t.Fatalf("expected variable 'result', got %s", fa.Variable)
	}
	if fa.Error != "err" {
		t.Fatalf("expected error 'err', got %s", fa.Error)
	}
}

func TestFallibleAssignment_Direct(t *testing.T) {
	cmd := mustParseLine(t, "$val, $err := get account test")
	fa, ok := cmd.(FallibleAssignment)
	if !ok {
		t.Fatalf("expected FallibleAssignment, got %T", cmd)
	}
	if !fa.Direct {
		t.Fatalf("expected Direct=true")
	}
}

// ---------------------------------------------------------------------------
// Approve / Submit / Cancel
// ---------------------------------------------------------------------------

func TestApprove(t *testing.T) {
	cmd := mustParseLine(t, "approve $myxfer")
	a, ok := cmd.(Approve)
	if !ok {
		t.Fatalf("expected Approve, got %T", cmd)
	}
	if !a.Target.IsVariable || a.Target.VarName != "myxfer" {
		t.Fatalf("unexpected target: %v", a.Target)
	}
}

func TestSubmit(t *testing.T) {
	cmd := mustParseLine(t, "submit $myxfer")
	s, ok := cmd.(Submit)
	if !ok {
		t.Fatalf("expected Submit, got %T", cmd)
	}
	if !s.Target.IsVariable || s.Target.VarName != "myxfer" {
		t.Fatalf("unexpected target: %v", s.Target)
	}
}

func TestCancel(t *testing.T) {
	cmd := mustParseLine(t, "cancel $myxfer")
	c, ok := cmd.(Cancel)
	if !ok {
		t.Fatalf("expected Cancel, got %T", cmd)
	}
	if !c.Target.IsVariable || c.Target.VarName != "myxfer" {
		t.Fatalf("unexpected target: %v", c.Target)
	}
}

// ---------------------------------------------------------------------------
// For loop
// ---------------------------------------------------------------------------

func TestForLoop_Range(t *testing.T) {
	cmd := mustParseLine(t, "for i in 0..5 { create account }")
	fl, ok := cmd.(ForLoop)
	if !ok {
		t.Fatalf("expected ForLoop, got %T", cmd)
	}
	if fl.Variable != "i" {
		t.Fatalf("expected variable 'i', got %s", fl.Variable)
	}
	ri, ok := fl.Iterable.(RangeIterable)
	if !ok {
		t.Fatalf("expected RangeIterable, got %T", fl.Iterable)
	}
	if ri.Start != 0 || ri.End != 5 {
		t.Fatalf("expected 0..5, got %d..%d", ri.Start, ri.End)
	}
	if ri.Inclusive {
		t.Fatalf("expected non-inclusive range")
	}
	if _, ok := fl.Body.(Create); !ok {
		t.Fatalf("expected Create body, got %T", fl.Body)
	}
}

func TestForLoop_InclusiveRange(t *testing.T) {
	cmd := mustParseLine(t, "for i in 1..=10 { create account }")
	fl, ok := cmd.(ForLoop)
	if !ok {
		t.Fatalf("expected ForLoop, got %T", cmd)
	}
	ri, ok := fl.Iterable.(RangeIterable)
	if !ok {
		t.Fatalf("expected RangeIterable, got %T", fl.Iterable)
	}
	if !ri.Inclusive {
		t.Fatalf("expected inclusive range")
	}
	if ri.Start != 1 || ri.End != 10 {
		t.Fatalf("expected 1..=10")
	}
}

func TestForLoop_Array(t *testing.T) {
	cmd := mustParseLine(t, `for x in ["a", "b", "c"] { create account }`)
	fl, ok := cmd.(ForLoop)
	if !ok {
		t.Fatalf("expected ForLoop, got %T", cmd)
	}
	ai, ok := fl.Iterable.(ArrayIterable)
	if !ok {
		t.Fatalf("expected ArrayIterable, got %T", fl.Iterable)
	}
	if len(ai.Values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(ai.Values))
	}
}

func TestForLoop_ListIterable(t *testing.T) {
	cmd := mustParseLine(t, "for acct in list accounts { delete $acct }")
	fl, ok := cmd.(ForLoop)
	if !ok {
		t.Fatalf("expected ForLoop, got %T", cmd)
	}
	li, ok := fl.Iterable.(ListIterable)
	if !ok {
		t.Fatalf("expected ListIterable, got %T", fl.Iterable)
	}
	if li.List.ResourceType != ResourceAccount {
		t.Fatalf("expected Account list")
	}
	if _, ok := fl.Body.(Delete); !ok {
		t.Fatalf("expected Delete body, got %T", fl.Body)
	}
}

// ---------------------------------------------------------------------------
// Blueprint / Download / Upload
// ---------------------------------------------------------------------------

func TestBlueprint(t *testing.T) {
	cmd := mustParseLine(t, "blueprint standard my-invite-code")
	bp, ok := cmd.(Blueprint)
	if !ok {
		t.Fatalf("expected Blueprint, got %T", cmd)
	}
	if bp.Type != "standard" {
		t.Fatalf("expected type 'standard', got %s", bp.Type)
	}
	if bp.RootInvite != "my-invite-code" {
		t.Fatalf("expected root invite 'my-invite-code'")
	}
}

func TestDownloadTreasury(t *testing.T) {
	cmd := mustParseLine(t, "download treasury my-treasury ./output")
	dt, ok := cmd.(DownloadTreasury)
	if !ok {
		t.Fatalf("expected DownloadTreasury, got %T", cmd)
	}
	if dt.TreasuryId != "my-treasury" {
		t.Fatalf("expected treasury id 'my-treasury'")
	}
	if dt.Directory == nil || *dt.Directory != "./output" {
		t.Fatalf("expected directory './output'")
	}
}

func TestDownloadTreasury_NoDirectory(t *testing.T) {
	cmd := mustParseLine(t, "download treasury my-treasury")
	dt, ok := cmd.(DownloadTreasury)
	if !ok {
		t.Fatalf("expected DownloadTreasury, got %T", cmd)
	}
	if dt.Directory != nil {
		t.Fatalf("expected nil directory")
	}
}

func TestUploadBackup(t *testing.T) {
	cmd := mustParseLine(t, "upload backup /path/to/backup.tar.gz")
	ub, ok := cmd.(UploadBackup)
	if !ok {
		t.Fatalf("expected UploadBackup, got %T", cmd)
	}
	if ub.File != "/path/to/backup.tar.gz" {
		t.Fatalf("expected file path")
	}
}

// ---------------------------------------------------------------------------
// Convert / Replace
// ---------------------------------------------------------------------------

func TestConvert(t *testing.T) {
	cmd := mustParseLine(t, `convert $data from "hex" to "base64"`)
	cv, ok := cmd.(Convert)
	if !ok {
		t.Fatalf("expected Convert, got %T", cmd)
	}
	if vr, ok := cv.Data.(VariableRef); !ok || vr.Name != "data" {
		t.Fatalf("expected data=$data")
	}
	if sv, ok := cv.SourceEncoding.(StringValue); !ok || sv.Value != "hex" {
		t.Fatalf("expected source='hex'")
	}
	if sv, ok := cv.TargetEncoding.(StringValue); !ok || sv.Value != "base64" {
		t.Fatalf("expected target='base64'")
	}
}

func TestReplace(t *testing.T) {
	cmd := mustParseLine(t, `replace $data "old" "new"`)
	r, ok := cmd.(Replace)
	if !ok {
		t.Fatalf("expected Replace, got %T", cmd)
	}
	if vr, ok := r.Data.(VariableRef); !ok || vr.Name != "data" {
		t.Fatalf("expected data=$data")
	}
}

// ---------------------------------------------------------------------------
// Custom
// ---------------------------------------------------------------------------

func TestCustom_Variable(t *testing.T) {
	cmd := mustParseLine(t, "custom sign $myxfer")
	c, ok := cmd.(Custom)
	if !ok {
		t.Fatalf("expected Custom, got %T", cmd)
	}
	if c.Action != "sign" {
		t.Fatalf("expected action 'sign'")
	}
	if !c.Target.IsVariable || c.Target.VarName != "myxfer" {
		t.Fatalf("expected variable target")
	}
}

func TestCustom_WithTablePayload(t *testing.T) {
	cmd := mustParseLine(t, `custom sign $myxfer { key = "value" }`)
	c, ok := cmd.(Custom)
	if !ok {
		t.Fatalf("expected Custom, got %T", cmd)
	}
	if c.Payload == nil {
		t.Fatalf("expected payload")
	}
	if c.Payload.IsString {
		t.Fatalf("expected table payload, not string")
	}
	if c.Payload.TableValue == nil {
		t.Fatalf("expected table value")
	}
}

// ---------------------------------------------------------------------------
// Value parsing
// ---------------------------------------------------------------------------

func TestValueParsing_String(t *testing.T) {
	v, err := (&parser{}).parseValueFromString(`"hello world"`)
	if err != nil {
		t.Fatal(err)
	}
	sv, ok := v.(StringValue)
	if !ok {
		t.Fatalf("expected StringValue, got %T", v)
	}
	if sv.Value != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", sv.Value)
	}
}

func TestValueParsing_EscapedString(t *testing.T) {
	v, err := (&parser{}).parseValueFromString(`"he said \"hi\""`)
	if err != nil {
		t.Fatal(err)
	}
	sv, ok := v.(StringValue)
	if !ok {
		t.Fatalf("expected StringValue, got %T", v)
	}
	if sv.Value != `he said "hi"` {
		t.Fatalf("expected 'he said \"hi\"', got '%s'", sv.Value)
	}
}

func TestValueParsing_TripleQuotedString(t *testing.T) {
	v, err := (&parser{}).parseValueFromString(`"""content with "quotes" inside"""`)
	if err != nil {
		t.Fatal(err)
	}
	sv, ok := v.(StringValue)
	if !ok {
		t.Fatalf("expected StringValue, got %T", v)
	}
	if sv.Value != `content with "quotes" inside` {
		t.Fatalf("unexpected value: %s", sv.Value)
	}
}

func TestValueParsing_Integer(t *testing.T) {
	v, err := (&parser{}).parseValueFromString("42")
	if err != nil {
		t.Fatal(err)
	}
	iv, ok := v.(IntegerValue)
	if !ok {
		t.Fatalf("expected IntegerValue, got %T", v)
	}
	if iv.Value != 42 {
		t.Fatalf("expected 42, got %d", iv.Value)
	}
}

func TestValueParsing_NegativeInteger(t *testing.T) {
	v, err := (&parser{}).parseValueFromString("-5")
	if err != nil {
		t.Fatal(err)
	}
	iv, ok := v.(IntegerValue)
	if !ok {
		t.Fatalf("expected IntegerValue, got %T", v)
	}
	if iv.Value != -5 {
		t.Fatalf("expected -5, got %d", iv.Value)
	}
}

func TestValueParsing_Float(t *testing.T) {
	v, err := (&parser{}).parseValueFromString("3.14")
	if err != nil {
		t.Fatal(err)
	}
	fv, ok := v.(FloatValue)
	if !ok {
		t.Fatalf("expected FloatValue, got %T", v)
	}
	if fv.Value != 3.14 {
		t.Fatalf("expected 3.14, got %f", fv.Value)
	}
}

func TestValueParsing_Boolean(t *testing.T) {
	for _, tc := range []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"false", false},
	} {
		v, err := (&parser{}).parseValueFromString(tc.input)
		if err != nil {
			t.Fatal(err)
		}
		bv, ok := v.(BooleanValue)
		if !ok {
			t.Fatalf("expected BooleanValue, got %T", v)
		}
		if bv.Value != tc.expected {
			t.Fatalf("expected %v, got %v", tc.expected, bv.Value)
		}
	}
}

func TestValueParsing_Array(t *testing.T) {
	v, err := (&parser{}).parseValueFromString(`[1, "two", true]`)
	if err != nil {
		t.Fatal(err)
	}
	av, ok := v.(ArrayValue)
	if !ok {
		t.Fatalf("expected ArrayValue, got %T", v)
	}
	if len(av.Values) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(av.Values))
	}
	if _, ok := av.Values[0].(IntegerValue); !ok {
		t.Fatalf("expected first element IntegerValue")
	}
	if _, ok := av.Values[1].(StringValue); !ok {
		t.Fatalf("expected second element StringValue")
	}
	if _, ok := av.Values[2].(BooleanValue); !ok {
		t.Fatalf("expected third element BooleanValue")
	}
}

func TestValueParsing_EmptyArray(t *testing.T) {
	v, err := (&parser{}).parseValueFromString("[]")
	if err != nil {
		t.Fatal(err)
	}
	av, ok := v.(ArrayValue)
	if !ok {
		t.Fatalf("expected ArrayValue, got %T", v)
	}
	if av.Values != nil {
		t.Fatalf("expected nil values for empty array, got %v", av.Values)
	}
}

func TestValueParsing_VariableRef(t *testing.T) {
	v, err := (&parser{}).parseValueFromString("$myvar")
	if err != nil {
		t.Fatal(err)
	}
	vr, ok := v.(VariableRef)
	if !ok {
		t.Fatalf("expected VariableRef, got %T", v)
	}
	if vr.Name != "myvar" {
		t.Fatalf("expected 'myvar', got '%s'", vr.Name)
	}
	if len(vr.Path) != 0 {
		t.Fatalf("expected empty path")
	}
}

func TestValueParsing_VariableRefWithPath(t *testing.T) {
	v, err := (&parser{}).parseValueFromString("$account.name.first")
	if err != nil {
		t.Fatal(err)
	}
	vr, ok := v.(VariableRef)
	if !ok {
		t.Fatalf("expected VariableRef, got %T", v)
	}
	if vr.Name != "account" {
		t.Fatalf("expected 'account', got '%s'", vr.Name)
	}
	if len(vr.Path) != 2 || vr.Path[0] != "name" || vr.Path[1] != "first" {
		t.Fatalf("expected path [name, first], got %v", vr.Path)
	}
}

func TestValueParsing_FunctionCall(t *testing.T) {
	v, err := (&parser{}).parseValueFromString(`sha256("hello")`)
	if err != nil {
		t.Fatal(err)
	}
	fc, ok := v.(FunctionCall)
	if !ok {
		t.Fatalf("expected FunctionCall, got %T", v)
	}
	if fc.Name != "sha256" {
		t.Fatalf("expected 'sha256', got '%s'", fc.Name)
	}
	if len(fc.Args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(fc.Args))
	}
}

func TestValueParsing_FunctionCallMultipleArgs(t *testing.T) {
	v, err := (&parser{}).parseValueFromString(`verify("ed25519", $sig, $msg, $pub)`)
	if err != nil {
		t.Fatal(err)
	}
	fc, ok := v.(FunctionCall)
	if !ok {
		t.Fatalf("expected FunctionCall, got %T", v)
	}
	if fc.Name != "verify" {
		t.Fatalf("expected 'verify'")
	}
	if len(fc.Args) != 4 {
		t.Fatalf("expected 4 args, got %d", len(fc.Args))
	}
}

func TestValueParsing_Nonce(t *testing.T) {
	v, err := (&parser{}).parseValueFromString("nonce()")
	if err != nil {
		t.Fatal(err)
	}
	sv, ok := v.(StringValue)
	if !ok {
		t.Fatalf("expected StringValue (nonce is resolved at parse time), got %T", v)
	}
	if len(sv.Value) != 16 {
		t.Fatalf("expected 16-char hex nonce, got '%s' (len %d)", sv.Value, len(sv.Value))
	}
}

func TestValueParsing_Now(t *testing.T) {
	v, err := (&parser{}).parseValueFromString("now()")
	if err != nil {
		t.Fatal(err)
	}
	sv, ok := v.(StringValue)
	if !ok {
		t.Fatalf("expected StringValue (now is resolved at parse time), got %T", v)
	}
	if !strings.Contains(sv.Value, "T") {
		t.Fatalf("expected ISO timestamp, got '%s'", sv.Value)
	}
}

func TestValueParsing_Concat_Literals(t *testing.T) {
	v, err := (&parser{}).parseValueFromString(`concat("hello", " world")`)
	if err != nil {
		t.Fatal(err)
	}
	sv, ok := v.(StringValue)
	if !ok {
		t.Fatalf("expected StringValue (concat of literals resolved at parse time), got %T", v)
	}
	if sv.Value != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", sv.Value)
	}
}

func TestValueParsing_Concat_WithVariable(t *testing.T) {
	v, err := (&parser{}).parseValueFromString(`concat("prefix-", $var)`)
	if err != nil {
		t.Fatal(err)
	}
	fc, ok := v.(FunctionCall)
	if !ok {
		t.Fatalf("expected FunctionCall (deferred concat), got %T", v)
	}
	if fc.Name != "concat" {
		t.Fatalf("expected 'concat'")
	}
}

func TestValueParsing_Id(t *testing.T) {
	v, err := (&parser{}).parseValueFromString(`id("my-id")`)
	if err != nil {
		t.Fatal(err)
	}
	sv, ok := v.(StringValue)
	if !ok {
		t.Fatalf("expected StringValue (id returns its argument), got %T", v)
	}
	if sv.Value != "my-id" {
		t.Fatalf("expected 'my-id', got '%s'", sv.Value)
	}
}

func TestValueParsing_Hex(t *testing.T) {
	v, err := (&parser{}).parseValueFromString(`hex("abc")`)
	if err != nil {
		t.Fatal(err)
	}
	sv, ok := v.(StringValue)
	if !ok {
		t.Fatalf("expected StringValue, got %T", v)
	}
	if sv.Value != "616263" {
		t.Fatalf("expected '616263', got '%s'", sv.Value)
	}
}

func TestValueParsing_DelayedFunction(t *testing.T) {
	for _, name := range []string{"resource", "keccak256", "json", "typed_data", "parse_call"} {
		v, err := (&parser{}).parseValueFromString(name + `("test")`)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		fc, ok := v.(FunctionCall)
		if !ok {
			t.Fatalf("%s: expected FunctionCall, got %T", name, v)
		}
		if fc.Name != name {
			t.Fatalf("expected name '%s', got '%s'", name, fc.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// Inline table parsing
// ---------------------------------------------------------------------------

func TestInlineTable_Simple(t *testing.T) {
	v, err := (&parser{}).parseValueFromString(`{ name = "test", count = 42 }`)
	if err != nil {
		t.Fatal(err)
	}
	tbl, ok := v.(InlineTable)
	if !ok {
		t.Fatalf("expected InlineTable, got %T", v)
	}
	if len(tbl.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(tbl.Entries))
	}
	if tbl.Entries[0].Key != "name" {
		t.Fatalf("expected key 'name', got '%s'", tbl.Entries[0].Key)
	}
	if tbl.Entries[1].Key != "count" {
		t.Fatalf("expected key 'count', got '%s'", tbl.Entries[1].Key)
	}
}

func TestInlineTable_DottedKeys(t *testing.T) {
	v, err := (&parser{}).parseValueFromString(`{ notes.signer = "alice", notes.amount = 100 }`)
	if err != nil {
		t.Fatal(err)
	}
	tbl, ok := v.(InlineTable)
	if !ok {
		t.Fatalf("expected InlineTable, got %T", v)
	}
	if tbl.Entries[0].Key != "notes.signer" {
		t.Fatalf("expected dotted key 'notes.signer', got '%s'", tbl.Entries[0].Key)
	}
}

func TestInlineTable_QuotedKeys(t *testing.T) {
	v, err := (&parser{}).parseValueFromString(`{ "chains/SOL" = $addr }`)
	if err != nil {
		t.Fatal(err)
	}
	tbl, ok := v.(InlineTable)
	if !ok {
		t.Fatalf("expected InlineTable, got %T", v)
	}
	if tbl.Entries[0].Key != "chains/SOL" {
		t.Fatalf("expected key 'chains/SOL', got '%s'", tbl.Entries[0].Key)
	}
}

func TestInlineTable_VariableValues(t *testing.T) {
	v, err := (&parser{}).parseValueFromString(`{ name = $user.name, id = $user.id }`)
	if err != nil {
		t.Fatal(err)
	}
	tbl, ok := v.(InlineTable)
	if !ok {
		t.Fatalf("expected InlineTable, got %T", v)
	}
	vr, ok := tbl.Entries[0].Value.(VariableRef)
	if !ok {
		t.Fatalf("expected VariableRef value, got %T", tbl.Entries[0].Value)
	}
	if vr.Name != "user" || len(vr.Path) != 1 || vr.Path[0] != "name" {
		t.Fatalf("expected $user.name, got %v", vr)
	}
}

func TestInlineTable_BooleanValues(t *testing.T) {
	v, err := (&parser{}).parseValueFromString(`{ active = true, deleted = false }`)
	if err != nil {
		t.Fatal(err)
	}
	tbl, ok := v.(InlineTable)
	if !ok {
		t.Fatalf("expected InlineTable, got %T", v)
	}
	if bv, ok := tbl.Entries[0].Value.(BooleanValue); !ok || bv.Value != true {
		t.Fatalf("expected true")
	}
	if bv, ok := tbl.Entries[1].Value.(BooleanValue); !ok || bv.Value != false {
		t.Fatalf("expected false")
	}
}

func TestInlineTable_ArrayValues(t *testing.T) {
	v, err := (&parser{}).parseValueFromString(`{ tags = ["a", "b"] }`)
	if err != nil {
		t.Fatal(err)
	}
	tbl, ok := v.(InlineTable)
	if !ok {
		t.Fatalf("expected InlineTable, got %T", v)
	}
	av, ok := tbl.Entries[0].Value.(ArrayValue)
	if !ok {
		t.Fatalf("expected ArrayValue, got %T", tbl.Entries[0].Value)
	}
	if len(av.Values) != 2 {
		t.Fatalf("expected 2 array elements")
	}
}

func TestInlineTable_Empty(t *testing.T) {
	v, err := (&parser{}).parseValueFromString(`{}`)
	if err != nil {
		t.Fatal(err)
	}
	tbl, ok := v.(InlineTable)
	if !ok {
		t.Fatalf("expected InlineTable, got %T", v)
	}
	if len(tbl.Entries) != 0 {
		t.Fatalf("expected empty table")
	}
}

// ---------------------------------------------------------------------------
// Resource type APIPath
// ---------------------------------------------------------------------------

func TestResourceType_APIPath(t *testing.T) {
	tests := []struct {
		rt       ResourceType
		expected string
	}{
		{ResourceAccount, "accounts"},
		{ResourceAddress, "addresses"},
		{ResourceAccessRule, "access-rules"},
		{ResourceTransferRule, "transfer-rules"},
		{ResourceCallRule, "call-rules"},
		{ResourceStakingRule, "staking-rules"},
		{ResourceClientKey, "client-keys"},
		{ResourceSoftwareUpdate, "software-updates"},
		{ResourceTransfer, "transfers"},
		{ResourceTreasury, "treasuries"},
		{ResourceSignatory, "signatories"},
	}
	for _, tc := range tests {
		if got := tc.rt.APIPath(); got != tc.expected {
			t.Errorf("ResourceType(%d).APIPath() = %s, want %s", tc.rt, got, tc.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Full program parsing
// ---------------------------------------------------------------------------

func TestParse_MultiLineProgram(t *testing.T) {
	source := `#!/usr/bin/env csl
# Setup accounts
$acct = create internal account
$addr = create address for SOL

# Do a transfer
$xfer = create transfer for SOL { amount = 100, to = $addr.id }
approve $xfer
submit $xfer

# Cleanup
exit
`
	prog := mustParse(t, source)
	if prog == nil {
		t.Fatal("expected non-nil program")
	}

	// Count non-nop commands
	var nonNop int
	for _, cmd := range prog.Commands {
		if _, ok := cmd.(Nop); !ok {
			nonNop++
		}
	}
	if nonNop != 6 {
		t.Fatalf("expected 6 non-Nop commands, got %d", nonNop)
	}
}

func TestParse_EmptyProgram(t *testing.T) {
	prog := mustParse(t, "")
	if len(prog.Commands) != 1 {
		t.Fatalf("expected 1 command (empty line -> Nop), got %d", len(prog.Commands))
	}
}

func TestParse_CommentsOnly(t *testing.T) {
	source := `# comment 1
// comment 2
# comment 3`
	prog := mustParse(t, source)
	for _, cmd := range prog.Commands {
		if _, ok := cmd.(Nop); !ok {
			t.Fatalf("expected all Nop, got %T", cmd)
		}
	}
}

// ---------------------------------------------------------------------------
// Tokeniser
// ---------------------------------------------------------------------------

func TestTokeniseLine_Simple(t *testing.T) {
	tokens := tokeniseLine("create account my-id")
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0] != "create" || tokens[1] != "account" || tokens[2] != "my-id" {
		t.Fatalf("unexpected tokens: %v", tokens)
	}
}

func TestTokeniseLine_QuotedString(t *testing.T) {
	tokens := tokeniseLine(`set name "hello world"`)
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[2] != `"hello world"` {
		t.Fatalf("expected quoted string token, got %s", tokens[2])
	}
}

func TestTokeniseLine_InlineTable(t *testing.T) {
	tokens := tokeniseLine(`create account { name = "test", x = 1 }`)
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
	if !strings.HasPrefix(tokens[2], "{") || !strings.HasSuffix(tokens[2], "}") {
		t.Fatalf("expected brace-delimited token, got %s", tokens[2])
	}
}

func TestTokeniseLine_FunctionCall(t *testing.T) {
	tokens := tokeniseLine(`$var = sha256("hello")`)
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[2] != `sha256("hello")` {
		t.Fatalf("expected function call token, got %s", tokens[2])
	}
}

func TestTokeniseLine_ArrayLiteral(t *testing.T) {
	tokens := tokeniseLine(`$arr = ["a", "b", "c"]`)
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
	if !strings.HasPrefix(tokens[2], "[") {
		t.Fatalf("expected array token, got %s", tokens[2])
	}
}

// ---------------------------------------------------------------------------
// PartialOrVariable
// ---------------------------------------------------------------------------

func TestPartialOrVariable_String(t *testing.T) {
	pv := PartialOrVariable{IsVariable: true, VarName: "test"}
	if pv.String() != "$test" {
		t.Fatalf("expected '$test', got '%s'", pv.String())
	}

	pv2 := PartialOrVariable{ResourceType: ResourceAccount, Id: "my-acct"}
	s := pv2.String()
	if !strings.Contains(s, "account") || !strings.Contains(s, "my-acct") {
		t.Fatalf("unexpected string: %s", s)
	}
}

func TestIdOrVariable_String(t *testing.T) {
	iv := IdOrVariable{IsVariable: true, VarName: "x"}
	if iv.String() != "$x" {
		t.Fatalf("expected '$x', got '%s'", iv.String())
	}
	iv2 := IdOrVariable{Id: "ETH"}
	if iv2.String() != "ETH" {
		t.Fatalf("expected 'ETH', got '%s'", iv2.String())
	}
}

// ---------------------------------------------------------------------------
// ParseError
// ---------------------------------------------------------------------------

func TestParseError_Format(t *testing.T) {
	err := &ParseError{Line: 5, Column: 10, Message: "bad input"}
	s := err.Error()
	if !strings.Contains(s, "line 5") || !strings.Contains(s, "bad input") {
		t.Fatalf("unexpected error format: %s", s)
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestCreate_SoftwareUpdate(t *testing.T) {
	cmd := mustParseLine(t, "create software-update")
	c, ok := cmd.(Create)
	if !ok {
		t.Fatalf("expected Create, got %T", cmd)
	}
	if c.ResourceType != ResourceSoftwareUpdate {
		t.Fatalf("expected SoftwareUpdate, got %s", c.ResourceType)
	}
}

func TestCreate_ClientKey(t *testing.T) {
	cmd := mustParseLine(t, "create client-key")
	c, ok := cmd.(Create)
	if !ok {
		t.Fatalf("expected Create, got %T", cmd)
	}
	if c.ResourceType != ResourceClientKey {
		t.Fatalf("expected ClientKey, got %s", c.ResourceType)
	}
}

func TestList_TransferRulesMultiWord(t *testing.T) {
	cmd := mustParseLine(t, "list transfer rules")
	l, ok := cmd.(ListCmd)
	if !ok {
		t.Fatalf("expected ListCmd, got %T", cmd)
	}
	if l.ResourceType != ResourceTransferRule {
		t.Fatalf("expected TransferRule, got %s", l.ResourceType)
	}
}

func TestList_StakingPolicy(t *testing.T) {
	cmd := mustParseLine(t, "list staking policy")
	l, ok := cmd.(ListCmd)
	if !ok {
		t.Fatalf("expected ListCmd, got %T", cmd)
	}
	if l.ResourceType != ResourceStakingPolicy {
		t.Fatalf("expected StakingPolicy, got %s", l.ResourceType)
	}
}

func TestAssignment_VariableRef(t *testing.T) {
	cmd := mustParseLine(t, "$x = $other.field")
	a, ok := cmd.(Assignment)
	if !ok {
		t.Fatalf("expected Assignment, got %T", cmd)
	}
	va, ok := a.Value.(ValueAssignable)
	if !ok {
		t.Fatalf("expected ValueAssignable, got %T", a.Value)
	}
	vr, ok := va.Value.(VariableRef)
	if !ok {
		t.Fatalf("expected VariableRef, got %T", va.Value)
	}
	if vr.Name != "other" || len(vr.Path) != 1 || vr.Path[0] != "field" {
		t.Fatalf("expected $other.field, got %v", vr)
	}
}

func TestAssignment_Boolean(t *testing.T) {
	cmd := mustParseLine(t, "$flag = true")
	a, ok := cmd.(Assignment)
	if !ok {
		t.Fatalf("expected Assignment, got %T", cmd)
	}
	va, ok := a.Value.(ValueAssignable)
	if !ok {
		t.Fatalf("expected ValueAssignable, got %T", a.Value)
	}
	bv, ok := va.Value.(BooleanValue)
	if !ok {
		t.Fatalf("expected BooleanValue, got %T", va.Value)
	}
	if bv.Value != true {
		t.Fatalf("expected true")
	}
}

func TestCreate_WithVariableId(t *testing.T) {
	cmd := mustParseLine(t, "create account $myid")
	c, ok := cmd.(Create)
	if !ok {
		t.Fatalf("expected Create, got %T", cmd)
	}
	if c.Id == nil || *c.Id != "myid" {
		t.Fatalf("expected id 'myid', got %v", c.Id)
	}
}

func TestCreate_WithVariableParent(t *testing.T) {
	cmd := mustParseLine(t, "create address for $chain")
	c, ok := cmd.(Create)
	if !ok {
		t.Fatalf("expected Create, got %T", cmd)
	}
	if c.Parent == nil || *c.Parent != "chain" {
		t.Fatalf("expected parent 'chain', got %v", c.Parent)
	}
}

func TestCreate_StakingVariant(t *testing.T) {
	cmd := mustParseLine(t, "create stake staking")
	c, ok := cmd.(Create)
	if !ok {
		t.Fatalf("expected Create, got %T", cmd)
	}
	if c.ResourceType != ResourceStaking {
		t.Fatalf("expected Staking, got %s", c.ResourceType)
	}
	if c.Variant == nil || *c.Variant != "stake" {
		t.Fatalf("expected variant 'stake'")
	}
}

func TestGet_ResourceWithParent(t *testing.T) {
	cmd := mustParseLine(t, "get address my-addr for ETH")
	g, ok := cmd.(Get)
	if !ok {
		t.Fatalf("expected Get, got %T", cmd)
	}
	if g.Target.ResourceType != ResourceAddress {
		t.Fatalf("expected Address")
	}
	if g.Target.Id != "my-addr" {
		t.Fatalf("expected id 'my-addr'")
	}
	if g.Target.Parent != "ETH" {
		t.Fatalf("expected parent 'ETH', got '%s'", g.Target.Parent)
	}
}

// ---------------------------------------------------------------------------
// Command type strings
// ---------------------------------------------------------------------------

func TestCommandTypeStrings(t *testing.T) {
	tests := []struct {
		cmd      Command
		expected string
	}{
		{Nop{}, "Nop"},
		{Exit{}, "Exit"},
		{ExitIfSet{}, "ExitIfSet"},
		{SetSetting{}, "SetSetting"},
		{GetSetting{}, "GetSetting"},
		{UnsetSetting{}, "UnsetSetting"},
		{Assignment{}, "Assignment"},
		{FallibleAssignment{}, "FallibleAssignment"},
		{Create{}, "Create"},
		{Propose{}, "Propose"},
		{Update{}, "Update"},
		{Delete{}, "Delete"},
		{Get{}, "Get"},
		{ListCmd{}, "List"},
		{Assert{}, "Assert"},
		{Until{}, "Until"},
		{ForLoop{}, "ForLoop"},
		{Custom{}, "Custom"},
		{Approve{}, "Approve"},
		{Submit{}, "Submit"},
		{Cancel{}, "Cancel"},
		{Blueprint{}, "Blueprint"},
		{DownloadTreasury{}, "DownloadTreasury"},
		{UploadBackup{}, "UploadBackup"},
		{ValueCmd{}, "Value"},
		{Convert{}, "Convert"},
		{Replace{}, "Replace"},
	}
	for _, tc := range tests {
		if got := tc.cmd.commandType(); got != tc.expected {
			t.Errorf("%T.commandType() = %s, want %s", tc.cmd, got, tc.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Value type strings
// ---------------------------------------------------------------------------

func TestValueTypeStrings(t *testing.T) {
	tests := []struct {
		val      Value
		expected string
	}{
		{StringValue{}, "String"},
		{IntegerValue{}, "Integer"},
		{FloatValue{}, "Float"},
		{BooleanValue{}, "Boolean"},
		{ArrayValue{}, "Array"},
		{InlineTable{}, "InlineTable"},
		{VariableRef{}, "VariableRef"},
		{FunctionCall{}, "FunctionCall"},
	}
	for _, tc := range tests {
		if got := tc.val.valueType(); got != tc.expected {
			t.Errorf("%T.valueType() = %s, want %s", tc.val, got, tc.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Iterable type strings
// ---------------------------------------------------------------------------

func TestIterableTypeStrings(t *testing.T) {
	tests := []struct {
		it       Iterable
		expected string
	}{
		{ArrayIterable{}, "Array"},
		{RangeIterable{}, "Range"},
		{ListIterable{}, "List"},
	}
	for _, tc := range tests {
		if got := tc.it.iterableType(); got != tc.expected {
			t.Errorf("%T.iterableType() = %s, want %s", tc.it, got, tc.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Assignable type strings
// ---------------------------------------------------------------------------

func TestAssignableTypeStrings(t *testing.T) {
	va := ValueAssignable{Value: StringValue{Value: "x"}}
	if va.assignableType() != "Value" {
		t.Fatal("expected 'Value'")
	}
	ca := CommandAssignable{Cmd: Nop{}}
	if ca.assignableType() != "Command" {
		t.Fatal("expected 'Command'")
	}
}

// ---------------------------------------------------------------------------
// Error cases
// ---------------------------------------------------------------------------

func TestError_ExitIfSetNoArg(t *testing.T) {
	_, err := ParseLine("exit_if_set")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestError_SetNoValue(t *testing.T) {
	_, err := ParseLine("set key")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestError_CreateNoResource(t *testing.T) {
	_, err := ParseLine("create")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestError_DeleteNoTarget(t *testing.T) {
	_, err := ParseLine("delete")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestError_GetNoTarget(t *testing.T) {
	_, err := ParseLine("get")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestError_ListNoResource(t *testing.T) {
	_, err := ParseLine("list")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestError_BlueprintNoArgs(t *testing.T) {
	_, err := ParseLine("blueprint")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestError_ConvertBadSyntax(t *testing.T) {
	_, err := ParseLine("convert $data")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestError_AssignmentNoOperator(t *testing.T) {
	_, err := ParseLine("$var")
	if err == nil {
		t.Fatal("expected error")
	}
}
