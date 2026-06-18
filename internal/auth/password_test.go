package auth

import "testing"

func TestHashPasswordAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("secret-pass")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" || hash == "secret-pass" {
		t.Fatal("expected non-empty bcrypt hash")
	}
	if !CheckPassword(hash, "secret-pass") {
		t.Fatal("CheckPassword should succeed for correct password")
	}
	if CheckPassword(hash, "wrong-pass") {
		t.Fatal("CheckPassword should fail for wrong password")
	}
}
