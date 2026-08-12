package postaccess

import "testing"

func TestDigestMatchesPHP(t *testing.T) {
	// hash('sha256', 'door-code') as produced by the PHP that wrote all 967 rows.
	const fromPHP = "4a2e0e524baa762d413f3b897a76fa20bc483c703e2a55a9469e0419ae39a25c"
	if got := HashPassword("door-code"); got != fromPHP {
		t.Fatalf("Go digest = %s\nPHP digest = %s", got, fromPHP)
	}
	if !PasswordMatches("door-code", fromPHP) {
		t.Error("PasswordMatches() refused the digest PHP wrote")
	}
}
