package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cordialsys/sdk-go/csl"
	"github.com/cordialsys/sdk-go/treasury/client"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "script":
		runScript()
	case "test":
		runTest()
	case "get":
		runGet()
	case "list", "ls":
		runList()
	case "create":
		runCreate()
	case "update":
		runUpdate()
	case "delete":
		runDelete()
	case "treasury":
		runTreasury()
	case "config":
		runConfig()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`treasury-go - Go SDK CLI for Cordial Treasury

Usage:
  treasury-go <command> [options]

Commands:
  script   Run a CSL script file
  test     Run CSL test suite
  get      Get a resource by name
  list     List resources of a type
  create   Create a resource
  update   Update a resource
  delete   Delete a resource
  treasury List/get treasury instances
  config   Show configuration

Options:
  -a, --api <url>       API base URL (default: http://localhost:8777)
  -t, --treasury <id>   Treasury ID
  -s, --sign-with <key> Keyring key name to sign with
  -f, --file <path>     Script file to run (for script/test commands)
  --suite <dir>         Test suite directory (for test command)`)
}

// parseFlags extracts common flags from os.Args[2:]
type flags struct {
	apiURL    string
	treasury  string
	signWith  string
	file      string
	suite     string
	filter    string
	remaining []string
}

func parseFlags() flags {
	f := flags{
		apiURL: os.Getenv("TREASURY_API_URL"),
	}
	if f.apiURL == "" {
		f.apiURL = "http://localhost:8777"
	}
	f.treasury = os.Getenv("TREASURY_ID")

	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-a", "--api":
			if i+1 < len(args) {
				f.apiURL = args[i+1]
				i++
			}
		case "-t", "--treasury":
			if i+1 < len(args) {
				f.treasury = args[i+1]
				i++
			}
		case "-s", "--sign-with":
			if i+1 < len(args) {
				f.signWith = args[i+1]
				i++
			}
		case "-f", "--file":
			if i+1 < len(args) {
				f.file = args[i+1]
				i++
			}
		case "--suite":
			if i+1 < len(args) {
				f.suite = args[i+1]
				i++
			}
		case "--filter":
			if i+1 < len(args) {
				f.filter = args[i+1]
				i++
			}
		default:
			f.remaining = append(f.remaining, args[i])
		}
	}
	return f
}

func makeClient(f flags) *client.Client {
	treasuryID := f.treasury
	if treasuryID == "" {
		// Try to auto-discover treasury ID
		c := client.NewClient(f.apiURL, "")
		raw, err := c.GetJSON("treasury")
		if err == nil {
			var t struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(raw, &t) == nil && t.Name != "" {
				parts := strings.Split(t.Name, "/")
				if len(parts) >= 2 {
					treasuryID = parts[1]
				}
			}
		}
	}
	return client.NewClient(f.apiURL, treasuryID)
}

func makeVM(f flags) *csl.VM {
	c := makeClient(f)
	vm := csl.NewVM(c)

	keyring, _ := client.NewKeyring(c.TreasuryID)
	vm.Keyring = keyring

	if f.signWith != "" && keyring != nil {
		identity, err := keyring.GetKey(f.signWith)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading key %q: %v\n", f.signWith, err)
			os.Exit(1)
		}
		c.SetIdentity(identity)
	}

	return vm
}

func runScript() {
	f := parseFlags()
	if f.file == "" && len(f.remaining) > 0 {
		f.file = f.remaining[0]
	}

	var source string
	if f.file == "" || f.file == "-" {
		// Read from stdin
		data, err := os.ReadFile("/dev/stdin")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
			os.Exit(1)
		}
		source = string(data)
	} else {
		data, err := os.ReadFile(f.file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading file %s: %v\n", f.file, err)
			os.Exit(1)
		}
		source = string(data)
	}

	prog, err := csl.Parse(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}

	vm := makeVM(f)
	if err := vm.Execute(prog); err != nil {
		fmt.Fprintf(os.Stderr, "execution error: %v\n", err)
		os.Exit(1)
	}
}

func runTest() {
	f := parseFlags()

	// Collect test files
	var files []string

	if f.suite != "" {
		entries, err := os.ReadDir(f.suite)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading suite directory %s: %v\n", f.suite, err)
			os.Exit(1)
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".csl") {
				files = append(files, f.suite+"/"+e.Name())
			}
		}
	}
	if f.file != "" {
		files = append(files, f.file)
	}
	if len(f.remaining) > 0 {
		for _, arg := range f.remaining {
			info, err := os.Stat(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			if info.IsDir() {
				entries, err := os.ReadDir(arg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error reading directory %s: %v\n", arg, err)
					os.Exit(1)
				}
				for _, e := range entries {
					if !e.IsDir() && strings.HasSuffix(e.Name(), ".csl") {
						files = append(files, arg+"/"+e.Name())
					}
				}
			} else {
				files = append(files, arg)
			}
		}
	}

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no test files specified")
		os.Exit(1)
	}

	passed := 0
	failed := 0
	var failures []string

	for _, file := range files {
		name := file
		fmt.Printf(":: %s ... ", name)

		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("FAIL (read error: %v)\n", err)
			failed++
			failures = append(failures, name)
			continue
		}

		prog, err := csl.Parse(string(data))
		if err != nil {
			fmt.Printf("FAIL (parse error: %v)\n", err)
			failed++
			failures = append(failures, name)
			continue
		}

		vm := makeVM(f)
		if err := vm.Execute(prog); err != nil {
			fmt.Printf("FAIL (%v)\n", err)
			failed++
			failures = append(failures, name)
			continue
		}

		fmt.Println("PASS")
		passed++
	}

	fmt.Printf("\n--- Results: %d passed, %d failed, %d total ---\n", passed, failed, passed+failed)
	if len(failures) > 0 {
		fmt.Println("Failures:")
		for _, f := range failures {
			fmt.Printf("  - %s\n", f)
		}
		os.Exit(1)
	}
}

func runGet() {
	f := parseFlags()
	if len(f.remaining) == 0 {
		fmt.Fprintln(os.Stderr, "usage: treasury-go get <resource-name>")
		os.Exit(1)
	}

	c := makeClient(f)
	if f.signWith != "" {
		keyring, _ := client.NewKeyring(c.TreasuryID)
		identity, err := keyring.GetKey(f.signWith)
		if err == nil {
			c.SetIdentity(identity)
		}
	}

	name := strings.Join(f.remaining, "/")
	raw, err := c.GetJSON(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var pretty json.RawMessage
	if json.Unmarshal(raw, &pretty) == nil {
		out, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Println(string(out))
	} else {
		fmt.Println(string(raw))
	}
}

func runList() {
	f := parseFlags()
	if len(f.remaining) == 0 {
		fmt.Fprintln(os.Stderr, "usage: treasury-go list <resource-type> [--filter <expr>]")
		os.Exit(1)
	}

	c := makeClient(f)
	resourceType := f.remaining[0]

	opts := client.ListOptions{
		Filter: f.filter,
	}
	resp, err := c.List(resourceType, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	for _, r := range resp.Resources {
		var pretty json.RawMessage = r
		out, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Println(string(out))
	}
}

func runCreate() {
	f := parseFlags()
	if len(f.remaining) < 1 {
		fmt.Fprintln(os.Stderr, "usage: treasury-go create <resource-type> [id] [json-body]")
		os.Exit(1)
	}

	c := makeClient(f)
	if f.signWith != "" {
		keyring, _ := client.NewKeyring(c.TreasuryID)
		identity, err := keyring.GetKey(f.signWith)
		if err == nil {
			c.SetIdentity(identity)
		}
	}

	resourceType := f.remaining[0]
	id := ""
	bodyStr := "{}"
	if len(f.remaining) > 1 {
		id = f.remaining[1]
	}
	if len(f.remaining) > 2 {
		bodyStr = f.remaining[2]
	}

	var body interface{}
	if err := json.Unmarshal([]byte(bodyStr), &body); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing body: %v\n", err)
		os.Exit(1)
	}

	opName, err := c.Create(resourceType, id, body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(opName)
}

func runUpdate() {
	f := parseFlags()
	if len(f.remaining) < 2 {
		fmt.Fprintln(os.Stderr, "usage: treasury-go update <resource-name> <json-body>")
		os.Exit(1)
	}

	c := makeClient(f)
	if f.signWith != "" {
		keyring, _ := client.NewKeyring(c.TreasuryID)
		identity, err := keyring.GetKey(f.signWith)
		if err == nil {
			c.SetIdentity(identity)
		}
	}

	name := f.remaining[0]
	bodyStr := f.remaining[1]

	var body interface{}
	if err := json.Unmarshal([]byte(bodyStr), &body); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing body: %v\n", err)
		os.Exit(1)
	}

	opName, err := c.Update(name, body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(opName)
}

func runDelete() {
	f := parseFlags()
	if len(f.remaining) < 1 {
		fmt.Fprintln(os.Stderr, "usage: treasury-go delete <resource-name>")
		os.Exit(1)
	}

	c := makeClient(f)
	if f.signWith != "" {
		keyring, _ := client.NewKeyring(c.TreasuryID)
		identity, err := keyring.GetKey(f.signWith)
		if err == nil {
			c.SetIdentity(identity)
		}
	}

	name := strings.Join(f.remaining, "/")
	opName, err := c.Delete(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(opName)
}

func runTreasury() {
	f := parseFlags()
	subCmd := "list"
	if len(f.remaining) > 0 {
		subCmd = f.remaining[0]
	}

	c := makeClient(f)

	switch subCmd {
	case "list", "ls":
		raw, err := c.GetJSON("treasuries")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		out, _ := json.MarshalIndent(json.RawMessage(raw), "", "  ")
		fmt.Println(string(out))

	case "get":
		raw, err := c.GetJSON("treasury")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		out, _ := json.MarshalIndent(json.RawMessage(raw), "", "  ")
		fmt.Println(string(out))

	default:
		fmt.Fprintf(os.Stderr, "unknown treasury subcommand: %s\n", subCmd)
		os.Exit(1)
	}
}

func runConfig() {
	f := parseFlags()
	fmt.Printf("API URL:  %s\n", f.apiURL)
	fmt.Printf("Treasury: %s\n", f.treasury)
	if f.signWith != "" {
		fmt.Printf("Sign with: %s\n", f.signWith)
	}
}
