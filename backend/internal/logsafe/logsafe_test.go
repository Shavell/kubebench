package logsafe

import "testing"

func TestText(t *testing.T) {
	cases := map[string]string{
		"pods/web":                        "pods/web",
		"x\n2026/11/26 [audit] forged":    `x\n2026/11/26 [audit] forged`,
		"a\r\nb":                          `a\r\nb`,
		"":                                "",
		"tab\tstays and unicode stays: ı": "tab\tstays and unicode stays: ı",
	}
	for in, want := range cases {
		if got := Text(in); got != want {
			t.Errorf("Text(%q) = %q, want %q", in, got, want)
		}
	}
}
