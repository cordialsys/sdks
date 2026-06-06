package client

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	dcrdecdsa "github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

// SigningAlgorithm represents a supported signing algorithm.
type SigningAlgorithm string

const (
	// AlgoEcdsaK256Sha256 is secp256k1 ECDSA with SHA-256.
	AlgoEcdsaK256Sha256 SigningAlgorithm = "ecdsa-k256-sha256"
	// AlgoEcdsaP256Sha256 is NIST P-256 ECDSA with SHA-256.
	AlgoEcdsaP256Sha256 SigningAlgorithm = "ecdsa-p256-sha256"
	// AlgoEd25519 is Ed25519.
	AlgoEd25519 SigningAlgorithm = "ed25519"
)

// Identity represents a signing identity used to authenticate write requests.
type Identity struct {
	// UserName is the treasury user name (e.g. "admin").
	UserName string
	// PublicKeyHex is the hex-encoded public key.
	PublicKeyHex string
	// Algorithm is the signing algorithm.
	Algorithm SigningAlgorithm
	// k256Key is the secp256k1 private key (set when Algorithm is ecdsa-k256-sha256).
	k256Key *secp256k1.PrivateKey
	// p256Key is the NIST P-256 private key (set when Algorithm is ecdsa-p256-sha256).
	p256Key *ecdsa.PrivateKey
	// ed25519Key is the Ed25519 private key (set when Algorithm is ed25519).
	ed25519Key ed25519.PrivateKey
}

// GenerateK256Identity generates a new secp256k1 identity.
func GenerateK256Identity(userName string) (*Identity, error) {
	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("generating secp256k1 key: %w", err)
	}
	pubKeyBytes := privKey.PubKey().SerializeCompressed()
	return &Identity{
		UserName:     userName,
		PublicKeyHex: hex.EncodeToString(pubKeyBytes),
		Algorithm:    AlgoEcdsaK256Sha256,
		k256Key:      privKey,
	}, nil
}

// GenerateEd25519Identity generates a new Ed25519 identity.
func GenerateEd25519Identity(userName string) (*Identity, error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating ed25519 key: %w", err)
	}
	return &Identity{
		UserName:     userName,
		PublicKeyHex: hex.EncodeToString(pubKey),
		Algorithm:    AlgoEd25519,
		ed25519Key:   privKey,
	}, nil
}

// GenerateP256Identity generates a new NIST P-256 identity.
func GenerateP256Identity(userName string) (*Identity, error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating P256 key: %w", err)
	}
	pubKeyBytes := elliptic.MarshalCompressed(elliptic.P256(), privKey.PublicKey.X, privKey.PublicKey.Y)
	return &Identity{
		UserName:     userName,
		PublicKeyHex: hex.EncodeToString(pubKeyBytes),
		Algorithm:    AlgoEcdsaP256Sha256,
		p256Key:      privKey,
	}, nil
}

// LoadP256Identity loads a NIST P-256 identity from a hex-encoded private key scalar.
func LoadP256Identity(userName string, privateKeyHex string) (*Identity, error) {
	privKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("decoding private key hex: %w", err)
	}
	curve := elliptic.P256()
	privKey := new(ecdsa.PrivateKey)
	privKey.PublicKey.Curve = curve
	privKey.D = new(big.Int).SetBytes(privKeyBytes)
	privKey.PublicKey.X, privKey.PublicKey.Y = curve.ScalarBaseMult(privKeyBytes)
	pubKeyBytes := elliptic.MarshalCompressed(curve, privKey.PublicKey.X, privKey.PublicKey.Y)
	return &Identity{
		UserName:     userName,
		PublicKeyHex: hex.EncodeToString(pubKeyBytes),
		Algorithm:    AlgoEcdsaP256Sha256,
		p256Key:      privKey,
	}, nil
}

// LoadK256Identity loads a secp256k1 identity from a hex-encoded private key.
func LoadK256Identity(userName string, privateKeyHex string) (*Identity, error) {
	privKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("decoding private key hex: %w", err)
	}
	privKey := secp256k1.PrivKeyFromBytes(privKeyBytes)
	pubKeyBytes := privKey.PubKey().SerializeCompressed()
	return &Identity{
		UserName:     userName,
		PublicKeyHex: hex.EncodeToString(pubKeyBytes),
		Algorithm:    AlgoEcdsaK256Sha256,
		k256Key:      privKey,
	}, nil
}

// LoadEd25519Identity loads an Ed25519 identity from a hex-encoded private key (seed).
func LoadEd25519Identity(userName string, privateKeyHex string) (*Identity, error) {
	seed, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("decoding private key hex: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("invalid ed25519 seed length: got %d, want %d", len(seed), ed25519.SeedSize)
	}
	privKey := ed25519.NewKeyFromSeed(seed)
	pubKey := privKey.Public().(ed25519.PublicKey)
	return &Identity{
		UserName:     userName,
		PublicKeyHex: hex.EncodeToString(pubKey),
		Algorithm:    AlgoEd25519,
		ed25519Key:   privKey,
	}, nil
}

// PrivateKeyHex returns the hex-encoded private key (or seed for ed25519).
func (id *Identity) PrivateKeyHex() string {
	switch id.Algorithm {
	case AlgoEcdsaK256Sha256:
		if id.k256Key != nil {
			return hex.EncodeToString(id.k256Key.Serialize())
		}
	case AlgoEcdsaP256Sha256:
		if id.p256Key != nil {
			b := id.p256Key.D.Bytes()
			// Pad to 32 bytes
			padded := make([]byte, 32)
			copy(padded[32-len(b):], b)
			return hex.EncodeToString(padded)
		}
	case AlgoEd25519:
		if id.ed25519Key != nil {
			return hex.EncodeToString(id.ed25519Key.Seed())
		}
	}
	return ""
}

// Sign signs the given message bytes using the identity's private key.
// For secp256k1, the message is expected to already be a 32-byte SHA-256 hash.
// For ed25519, the message is signed directly.
func (id *Identity) Sign(message []byte) ([]byte, error) {
	switch id.Algorithm {
	case AlgoEcdsaK256Sha256:
		if id.k256Key == nil {
			return nil, fmt.Errorf("secp256k1 private key not set")
		}
		if len(message) != 32 {
			return nil, fmt.Errorf("secp256k1 signing requires a 32-byte hash, got %d bytes", len(message))
		}
		sig := dcrdecdsa.SignCompact(id.k256Key, message, false)
		// SignCompact returns [v, r(32), s(32)] = 65 bytes total.
		// The Treasury API expects r || s (64 bytes), without the recovery byte.
		if len(sig) == 65 {
			return sig[1:], nil
		}
		return sig, nil

	case AlgoEcdsaP256Sha256:
		if id.p256Key == nil {
			return nil, fmt.Errorf("P256 private key not set")
		}
		if len(message) != 32 {
			return nil, fmt.Errorf("P256 signing requires a 32-byte hash, got %d bytes", len(message))
		}
		r, s, err := ecdsa.Sign(rand.Reader, id.p256Key, message)
		if err != nil {
			return nil, fmt.Errorf("P256 signing: %w", err)
		}
		// Encode as r || s, each 32 bytes
		rBytes := r.Bytes()
		sBytes := s.Bytes()
		sig := make([]byte, 64)
		copy(sig[32-len(rBytes):32], rBytes)
		copy(sig[64-len(sBytes):], sBytes)
		return sig, nil

	case AlgoEd25519:
		if id.ed25519Key == nil {
			return nil, fmt.Errorf("ed25519 private key not set")
		}
		return ed25519.Sign(id.ed25519Key, message), nil

	default:
		return nil, fmt.Errorf("unsupported signing algorithm: %s", id.Algorithm)
	}
}

// Verify verifies a signature against the given message using the identity's public key.
func (id *Identity) Verify(message, signature []byte) (bool, error) {
	switch id.Algorithm {
	case AlgoEcdsaK256Sha256:
		pubKeyBytes, err := hex.DecodeString(id.PublicKeyHex)
		if err != nil {
			return false, fmt.Errorf("decoding public key: %w", err)
		}
		pubKey, err := secp256k1.ParsePubKey(pubKeyBytes)
		if err != nil {
			return false, fmt.Errorf("parsing secp256k1 public key: %w", err)
		}
		if len(signature) != 64 {
			return false, fmt.Errorf("expected 64-byte signature, got %d", len(signature))
		}
		r := new(secp256k1.ModNScalar)
		r.SetByteSlice(signature[:32])
		s := new(secp256k1.ModNScalar)
		s.SetByteSlice(signature[32:])
		ecSig := dcrdecdsa.NewSignature(r, s)
		return ecSig.Verify(message, pubKey), nil

	case AlgoEcdsaP256Sha256:
		pubKeyBytes, err := hex.DecodeString(id.PublicKeyHex)
		if err != nil {
			return false, fmt.Errorf("decoding public key: %w", err)
		}
		x, y := elliptic.UnmarshalCompressed(elliptic.P256(), pubKeyBytes)
		if x == nil {
			return false, fmt.Errorf("invalid P256 public key")
		}
		pubKey := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
		if len(signature) != 64 {
			return false, fmt.Errorf("expected 64-byte signature, got %d", len(signature))
		}
		r := new(big.Int).SetBytes(signature[:32])
		s := new(big.Int).SetBytes(signature[32:])
		return ecdsa.Verify(pubKey, message, r, s), nil

	case AlgoEd25519:
		pubKeyBytes, err := hex.DecodeString(id.PublicKeyHex)
		if err != nil {
			return false, fmt.Errorf("decoding public key: %w", err)
		}
		if len(pubKeyBytes) != ed25519.PublicKeySize {
			return false, fmt.Errorf("invalid ed25519 public key length: %d", len(pubKeyBytes))
		}
		return ed25519.Verify(ed25519.PublicKey(pubKeyBytes), message, signature), nil

	default:
		return false, fmt.Errorf("unsupported signing algorithm: %s", id.Algorithm)
	}
}

// SignHTTPMessage signs a signature base string according to the HTTP Message
// Signatures specification (RFC 9421). For ECDSA (k256, p256), it SHA-256 hashes the
// message first, then signs the hash. For ed25519, it signs the raw bytes directly.
// Returns the raw signature bytes.
func (id *Identity) SignHTTPMessage(signatureBase []byte) ([]byte, error) {
	switch id.Algorithm {
	case AlgoEcdsaK256Sha256, AlgoEcdsaP256Sha256:
		hash := sha256.Sum256(signatureBase)
		return id.Sign(hash[:])
	case AlgoEd25519:
		return id.Sign(signatureBase)
	default:
		return nil, fmt.Errorf("unsupported signing algorithm: %s", id.Algorithm)
	}
}

// ComputeSignature computes the signature for a request body map.
// It serializes the request to JSON with the signature field set to empty string
// (canonical form), hashes it (for secp256k1) or signs directly (for ed25519),
// and returns the hex-encoded signature.
//
// Deprecated: This is used by the legacy JSON envelope signing. New code should
// use SignHTTPMessage for RFC 9421 HTTP Message Signatures.
func ComputeSignature(identity *Identity, reqBody map[string]interface{}) (string, error) {
	// Set signature to empty string for the canonical form used in signing.
	reqBody["signature"] = ""

	// json.Marshal produces keys in sorted (alphabetical) order, which matches
	// the canonical form expected by the Treasury API.
	canonical, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request for signing: %w", err)
	}

	var sigBytes []byte
	switch identity.Algorithm {
	case AlgoEcdsaK256Sha256, AlgoEcdsaP256Sha256:
		hash := sha256.Sum256(canonical)
		sigBytes, err = identity.Sign(hash[:])
		if err != nil {
			return "", fmt.Errorf("signing request: %w", err)
		}
	case AlgoEd25519:
		sigBytes, err = identity.Sign(canonical)
		if err != nil {
			return "", fmt.Errorf("signing request: %w", err)
		}
	default:
		return "", fmt.Errorf("unsupported algorithm: %s", identity.Algorithm)
	}

	return hex.EncodeToString(sigBytes), nil
}
