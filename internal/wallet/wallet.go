package wallet

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ripemd160"
	"golang.org/x/crypto/scrypt"
)

const (
	version            = byte(0x00)
	addressChecksumLen = 4
	// Scrypt parameters
	scryptN      = 32768
	scryptR      = 8
	scryptP      = 1
	scryptKeyLen = 32 // 32 bytes for AES-256
)

// Wallet stores private and public keys
type Wallet struct {
	PrivateKey ecdsa.PrivateKey
	PublicKey  []byte
}

// walletSerializable is used for wallet serialization
type walletSerializable struct {
	PrivateKeyD []byte
	PrivateKeyX []byte
	PrivateKeyY []byte
	PublicKey   []byte
}

func init() {
	gob.Register(elliptic.P256())
}

// NewWallet creates and returns a Wallet.
// Note: This function no longer saves the wallet. Saving is handled by SaveWallet, which requires a password.
func NewWallet() *Wallet {
	private, public := newKeyPair()
	wallet := Wallet{private, public}
	return &wallet
}

// LoadWallet loads and decrypts a wallet from a file using a password.
func LoadWallet(address string, password string) (*Wallet, error) {
	if !ValidateAddress(address) {
		return nil, fmt.Errorf("invalid address")
	}

	walletDir := getWalletDir()
	walletPath := filepath.Join(walletDir, fmt.Sprintf("%s.wallet", address))

	if _, err := os.Stat(walletPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("wallet not found")
	}

	fileContent, err := ioutil.ReadFile(walletPath)
	if err != nil {
		return nil, err
	}

	decryptedData, err := Decrypt(fileContent, password)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt wallet: %w", err)
	}

	var ws walletSerializable
	decoder := gob.NewDecoder(bytes.NewReader(decryptedData))
	if err := decoder.Decode(&ws); err != nil {
		return nil, err
	}

	curve := elliptic.P256()
	x := new(big.Int).SetBytes(ws.PrivateKeyX)
	y := new(big.Int).SetBytes(ws.PrivateKeyY)
	d := new(big.Int).SetBytes(ws.PrivateKeyD)

	privateKey := ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
			X:     x,
			Y:     y,
		},
		D: d,
	}

	return &Wallet{privateKey, ws.PublicKey}, nil
}

// SaveWallet encrypts and saves the wallet to a file.
func SaveWallet(address string, wallet *Wallet, password string) error {
	walletDir := getWalletDir()
	if err := os.MkdirAll(walletDir, 0700); err != nil {
		return err
	}

	walletPath := filepath.Join(walletDir, fmt.Sprintf("%s.wallet", address))

	ws := walletSerializable{
		PrivateKeyD: wallet.PrivateKey.D.Bytes(),
		PrivateKeyX: wallet.PrivateKey.X.Bytes(),
		PrivateKeyY: wallet.PrivateKey.Y.Bytes(),
		PublicKey:   wallet.PublicKey,
	}

	var content bytes.Buffer
	encoder := gob.NewEncoder(&content)
	if err := encoder.Encode(ws); err != nil {
		return err
	}

	encryptedContent, err := Encrypt(content.Bytes(), password)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(walletPath, encryptedContent, 0600)
}

// Encrypt data using AES-GCM with a key derived from a password via scrypt.
func Encrypt(data []byte, password string) ([]byte, error) {
	// Generate a random salt
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	// Derive key using scrypt
	key, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return nil, err
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Generate a random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	// Encrypt the data
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	// Prepend salt to the ciphertext
	return append(salt, ciphertext...), nil
}

// Decrypt data using AES-GCM with a key derived from a password via scrypt.
func Decrypt(data []byte, password string) ([]byte, error) {
	if len(data) < 32 {
		return nil, fmt.Errorf("invalid encrypted data")
	}
	// Extract salt and ciphertext
	salt, ciphertext := data[:32], data[32:]

	// Derive key using scrypt
	key, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return nil, err
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt the data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (check password)")
	}

	return plaintext, nil
}

// ListAddresses returns a list of addresses of all wallets
func ListAddresses() []string {
	var addresses []string
	walletDir := getWalletDir()

	files, err := ioutil.ReadDir(walletDir)
	if err != nil && !os.IsNotExist(err) {
		log.Panic(err)
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) == ".wallet" {
			address := f.Name()[:len(f.Name())-7] // Remove .wallet extension
			if ValidateAddress(address) {
				addresses = append(addresses, address)
			}
		}
	}

	return addresses
}

// GetAddress returns wallet address
func (w *Wallet) GetAddress() string {
	pubKeyHash := HashPubKey(w.PublicKey)

	versionedPayload := append([]byte{version}, pubKeyHash...)
	checksum := checksum(versionedPayload)

	fullPayload := append(versionedPayload, checksum...)
	address := hex.EncodeToString(fullPayload)

	return address
}

// HashPubKey hashes public key
func HashPubKey(pubKey []byte) []byte {
	publicSHA256 := sha256.Sum256(pubKey)

	RIPEMD160Hasher := ripemd160.New()
	_, err := RIPEMD160Hasher.Write(publicSHA256[:])
	if err != nil {
		log.Panic(err)
	}
	publicRIPEMD160 := RIPEMD160Hasher.Sum(nil)

	return publicRIPEMD160
}

// ValidateAddress check if address if valid
func ValidateAddress(address string) bool {
	pubKeyHash, err := hex.DecodeString(address)
	if err != nil {
		return false
	}

	if len(pubKeyHash) < addressChecksumLen+1 {
		return false
	}

	actualChecksum := pubKeyHash[len(pubKeyHash)-addressChecksumLen:]
	version := pubKeyHash[0]
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-addressChecksumLen]
	targetChecksum := checksum(append([]byte{version}, pubKeyHash...))

	return bytes.Compare(actualChecksum, targetChecksum) == 0
}

// SignData signs data using the wallet's private key
func (w *Wallet) SignData(data []byte) ([]byte, error) {
	r, s, err := ecdsa.Sign(rand.Reader, &w.PrivateKey, data)
	if err != nil {
		return nil, err
	}

	// Ensure each component is exactly 32 bytes
	rBytes := r.Bytes()
	sBytes := s.Bytes()

	// Pad with leading zeros if necessary
	rPadded := make([]byte, 32)
	sPadded := make([]byte, 32)

	copy(rPadded[32-len(rBytes):], rBytes)
	copy(sPadded[32-len(sBytes):], sBytes)

	signature := append(rPadded, sPadded...)
	return signature, nil
}

// VerifySignature verifies a signature against public key and data
func VerifySignature(pubKey []byte, data []byte, signature []byte) bool {
	if len(pubKey) != 64 {
		return false // Public key should be 64 bytes (32 bytes X + 32 bytes Y)
	}

	if len(signature) != 64 {
		return false
	}

	curve := elliptic.P256()
	x := new(big.Int).SetBytes(pubKey[:32])
	y := new(big.Int).SetBytes(pubKey[32:])

	publicKey := ecdsa.PublicKey{
		Curve: curve,
		X:     x,
		Y:     y,
	}

	rSign := new(big.Int).SetBytes(signature[:32])
	sSign := new(big.Int).SetBytes(signature[32:])

	return ecdsa.Verify(&publicKey, data, rSign, sSign)
}

// Checksum generates a checksum for a public key
func checksum(payload []byte) []byte {
	firstSHA := sha256.Sum256(payload)
	secondSHA := sha256.Sum256(firstSHA[:])

	return secondSHA[:addressChecksumLen]
}

// newKeyPair creates a new cryptographic key pair
func newKeyPair() (ecdsa.PrivateKey, []byte) {
	curve := elliptic.P256()
	private, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		log.Panic(err)
	}

	pubKey := append(private.PublicKey.X.Bytes(), private.PublicKey.Y.Bytes()...)
	return *private, pubKey
}

// getWalletDir returns the directory where wallet files are stored
func getWalletDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Panic(err)
	}
	return filepath.Join(homeDir, ".blockchain-wallets")
}
