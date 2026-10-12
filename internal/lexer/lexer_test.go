package lexer

import "testing"

func TestReadWhile(t *testing.T) {
	l := NewLexer("abc123def")

	got := l.ReadWhile(IsWordChar)
	if got != "abc123def" {
		t.Fatalf("ReadWhile = %q, want %q", got, "abc123def")
	}

	if !l.IsEOF() {
		t.Fatal("expected EOF after reading the whole input")
	}
}

func TestReadWhileStopsAtNonMatch(t *testing.T) {
	l := NewLexer("abc-def")

	got := l.ReadWhile(IsWordChar)
	if got != "abc" {
		t.Fatalf("ReadWhile = %q, want %q", got, "abc")
	}

	// The non-matching rune must have been backed up and be readable.
	r, ok := l.Read()
	if !ok || r != '-' {
		t.Fatalf("expected '-' after ReadWhile, got %q (ok=%v)", r, ok)
	}
}

func TestReadUntil(t *testing.T) {
	l := NewLexer("hello,world")

	got := l.ReadUntil(IsChar(','))
	if got != "hello," {
		t.Fatalf("ReadUntil = %q, want %q", got, "hello,")
	}
}

func TestReadWhileUnicode(t *testing.T) {
	// Multi-byte runes must round-trip correctly through the builder.
	l := NewLexer("héllo wörld")

	got := l.ReadWhile(IsWordChar)
	if got != "héllo" {
		t.Fatalf("ReadWhile = %q, want %q", got, "héllo")
	}
}

func TestReadInt(t *testing.T) {
	l := NewLexer("2024-01-02")

	n, ok := l.ReadInt()
	if !ok || n != 2024 {
		t.Fatalf("ReadInt = %d (ok=%v), want 2024", n, ok)
	}
}
