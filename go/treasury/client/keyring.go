package client

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Keyring manages client signing keys stored on disk.
// Keys are stored under the platform data directory as TOML files.
type Keyring struct {
	// Dir is the directory where key files are stored.
	Dir string
}

// NewKeyring creates a keyring for the given treasury ID.
// Keys are stored at {data_dir}/treasury/{treasuryID}/keyring/,
// matching the Rust Treasury CLI's layout.
func NewKeyring(treasuryID string) (*Keyring, error) {
	dataDir, err := dataDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(dataDir, "treasury", treasuryID, "keyring")
	return &Keyring{Dir: dir}, nil
}

func dataDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA is not set")
		}
		return appData, nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home directory: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support"), nil
	default:
		if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
			return xdgData, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home directory: %w", err)
		}
		return filepath.Join(home, ".local", "share"), nil
	}
}

// NewKeyringFromDir creates a keyring using a specific directory path.
func NewKeyringFromDir(dir string) *Keyring {
	return &Keyring{Dir: dir}
}

// ensureDir creates the keyring directory if it does not exist.
func (kr *Keyring) ensureDir() error {
	return os.MkdirAll(kr.Dir, 0700)
}

// keyPath returns the file path for a key with the given name.
func (kr *Keyring) keyPath(name string) string {
	return filepath.Join(kr.Dir, name+".toml")
}

// GenerateKey generates a new key with the given name and algorithm,
// and saves it to the keyring directory.
// Supported algorithms: "ecdsa-k256-sha256", "ed25519".
func (kr *Keyring) GenerateKey(name string, algorithm SigningAlgorithm) (*Identity, error) {
	if err := kr.ensureDir(); err != nil {
		return nil, fmt.Errorf("creating keyring directory: %w", err)
	}

	path := kr.keyPath(name)
	if _, err := os.Stat(path); err == nil {
		// Key already exists - load and return it
		existing, loadErr := kr.GetKey(name)
		if loadErr == nil && existing.Algorithm == algorithm {
			return existing, nil
		}
		// Algorithm mismatch or load error - remove and regenerate
		os.Remove(path)
	}

	var identity *Identity
	var err error
	switch algorithm {
	case AlgoEcdsaK256Sha256:
		identity, err = GenerateK256Identity(name)
	case AlgoEcdsaP256Sha256:
		identity, err = GenerateP256Identity(name)
	case AlgoEd25519:
		identity, err = GenerateEd25519Identity(name)
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}
	if err != nil {
		return nil, err
	}

	if err := kr.saveKey(name, identity); err != nil {
		return nil, err
	}
	return identity, nil
}

// SaveKey saves an existing identity to the keyring.
func (kr *Keyring) SaveKey(name string, identity *Identity) error {
	if err := kr.ensureDir(); err != nil {
		return fmt.Errorf("creating keyring directory: %w", err)
	}
	return kr.saveKey(name, identity)
}

// saveKey writes the key file in TOML format.
func (kr *Keyring) saveKey(name string, identity *Identity) error {
	path := kr.keyPath(name)

	content := fmt.Sprintf(
		"algorithm = %q\nsecret_key = %q\npublic_key = %q\n",
		string(identity.Algorithm),
		identity.PrivateKeyHex(),
		identity.PublicKeyHex,
	)

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("writing key file %s: %w", path, err)
	}
	return nil
}

// GetKey loads a key by name from the keyring directory.
func (kr *Keyring) GetKey(name string) (*Identity, error) {
	path := kr.keyPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading key file %s: %w", path, err)
	}
	return parseKeyTOML(name, string(data))
}

// ListKeys returns the names of all keys in the keyring.
func (kr *Keyring) ListKeys() ([]string, error) {
	entries, err := os.ReadDir(kr.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading keyring directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".toml") {
			names = append(names, strings.TrimSuffix(name, ".toml"))
		}
	}
	return names, nil
}

// DeleteKey removes a key from the keyring.
func (kr *Keyring) DeleteKey(name string) error {
	path := kr.keyPath(name)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing key file %s: %w", path, err)
	}
	return nil
}

// parseKeyTOML parses a simple TOML key file.
// Expected format:
//
//	algorithm = "ecdsa-k256-sha256"
//	secret_key = "hex..."
//	public_key = "hex..."
func parseKeyTOML(name, content string) (*Identity, error) {
	fields := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Remove surrounding quotes.
		value = strings.Trim(value, "\"")
		fields[key] = value
	}

	algorithm := fields["algorithm"]
	secretKey := fields["secret_key"]
	publicKey := fields["public_key"]

	if algorithm == "" || secretKey == "" {
		return nil, fmt.Errorf("key file for %q missing required fields (algorithm, secret_key)", name)
	}

	var identity *Identity
	var err error
	switch SigningAlgorithm(algorithm) {
	case AlgoEcdsaK256Sha256:
		identity, err = LoadK256Identity(name, secretKey)
	case AlgoEcdsaP256Sha256:
		identity, err = LoadP256Identity(name, secretKey)
	case AlgoEd25519:
		identity, err = LoadEd25519Identity(name, secretKey)
	default:
		return nil, fmt.Errorf("unsupported algorithm in key file: %s", algorithm)
	}
	if err != nil {
		return nil, fmt.Errorf("loading key %q: %w", name, err)
	}

	// If a public key was stored, verify it matches the derived one.
	if publicKey != "" && publicKey != identity.PublicKeyHex {
		return nil, fmt.Errorf("key %q: stored public key %s does not match derived public key %s",
			name, publicKey, identity.PublicKeyHex)
	}

	return identity, nil
}
