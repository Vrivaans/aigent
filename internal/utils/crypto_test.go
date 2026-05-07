package utils

import (
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := "12345678901234567890123456789012" // 32 bytes
	plaintext := "my-secret-api-key-12345"

	encrypted, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if encrypted == "" {
		t.Fatal("Encrypt returned empty string")
	}
	if encrypted == plaintext {
		t.Fatal("Encrypt returned plaintext without encryption")
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("Decrypt mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptShortKey(t *testing.T) {
	_, err := Encrypt("hello", "short")
	if err == nil {
		t.Fatal("Expected error with short key, got nil")
	}
}

func TestDecryptInvalidData(t *testing.T) {
	key := "12345678901234567890123456789012"
	_, err := Decrypt("not-valid-base64!!!", key)
	if err == nil {
		t.Fatal("Expected error with invalid base64, got nil")
	}
}

func TestDecryptGarbageCiphertext(t *testing.T) {
	key := "12345678901234567890123456789012"
	// Valid base64 but not valid AES-GCM ciphertext
	_, err := Decrypt("YWJjZGVmZ2hpamtsbW5vcA==", key)
	if err == nil {
		t.Fatal("Expected error with garbage ciphertext, got nil")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := "11111111111111111111111111111111"
	key2 := "22222222222222222222222222222222"
	plaintext := "secret"

	encrypted, err := Encrypt(plaintext, key1)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(encrypted, key2)
	if err == nil {
		t.Fatal("Expected error with wrong key, got nil")
	}
}

func TestEncryptDecryptEmptyString(t *testing.T) {
	key := "12345678901234567890123456789012"

	encrypted, err := Encrypt("", key)
	if err != nil {
		t.Fatalf("Encrypt empty string failed: %v", err)
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt empty string failed: %v", err)
	}
	if decrypted != "" {
		t.Fatalf("Expected empty string, got %q", decrypted)
	}
}
