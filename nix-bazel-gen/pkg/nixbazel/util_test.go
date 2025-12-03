package nixbazel

import (
	"testing"
)

func TestExtractHash(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/nix/store/x34bh6s6ighg7lb74nkjbx3nx52zj0j9-git-2.51.2", "x34bh6s6ighg7lb74nkjbx3nx52zj0j9"},
		{"x34bh6s6ighg7lb74nkjbx3nx52zj0j9-git-2.51.2", "x34bh6s6ighg7lb74nkjbx3nx52zj0j9"},
		{"/nix/store/invalid-path", ""},
		{"", ""},
	}

	for _, test := range tests {
		result := extractHash(test.input)
		if result != test.expected {
			t.Errorf("extractHash(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestConvertHashToHex(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"sha256:0gkxy2qfdi81lxzqbsdl2w5mdg0666s24inpa90ilvkb53ssmn3s", "7ad8aaf5286b6e1a4152d74622b43106bc560b17b4e9857fa701c5e6b0f07d3e"},
		{"sha256:1fd9f9qbrlckn0yapnybfv4y9bwwsvq473ckp71ccvvspr5602fy", "de09604abe7a6fc6c2b9938d43f0d69cafe4c976cbdbab3cb093d1bc7072a9b9"},
		{"invalid", ""},
	}

	for _, test := range tests {
		result := convertHashToHex(test.input)
		if result != test.expected {
			t.Errorf("convertHashToHex(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}
