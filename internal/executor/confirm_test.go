package executor

import "testing"

func TestStripControlChars_RemovesANSI(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"color code", "\x1b[31mred text\x1b[0m", "red text"},
		{"cursor move", "\x1b[2Ahidden", "hidden"},
		{"title set", "\x1b]0;fake title\x07real", "real"},
		{"bold", "\x1b[1mbold\x1b[22m", "bold"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stripControlChars(tc.input)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStripControlChars_RemovesNonPrintable(t *testing.T) {
	input := "hello\x00world\x01\x02end"
	got := stripControlChars(input)
	if got != "helloworldend" {
		t.Errorf("got %q, want %q", got, "helloworldend")
	}
}

func TestStripControlChars_PreservesNormalText(t *testing.T) {
	input := "Hello, World! 123 @#$ newline\ntab\there"
	got := stripControlChars(input)
	if got != input {
		t.Errorf("got %q, want %q", got, input)
	}
}
