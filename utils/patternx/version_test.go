package patternx

import (
	"testing"
)

func TestIsValidVersion(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"V1.0.0", true},
		{"V1", false},
		{"V1.0.0.0.0", false},
		{"1.0.0", true},
		{"V1.0.0-beta", false},
		{"V1234.5678.91011", false},
	}

	for _, c := range cases {
		got := IsValidVersion(c.version)
		if got != c.want {
			t.Errorf("IsValidVersion(%q) == %v, want %v", c.version, got, c.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		newVersion string
		oldVersion string
		want       int
		wantErr    bool
	}{
		{"V1.0.0", "V1.0.0", 0, false},
		{"V2.0.0", "V1.0.0", 1, false},
		{"V1.0.0", "V2.0.0", -1, false},
		{"V1.0.0", "V1.0", 1, false},
		{"V1.0", "V1.0.0", -1, false},
		{"V1.0.1", "V1.0.0", 1, false},
		{"V1.0.0", "V1.0.1", -1, false},
		{"V1.0.0", "V1.0.0.0", -1, false},
		{"V1.0.0.0", "V1.0.0", 1, false},
		{"V1.0.0", "V1.0.0-beta", 0, true},
		{"V1.0.0-beta", "V1.0.0", 0, true},
		{"V1.0.0", "1.0.0", 0, false},
		{"1.0.0", "V1.0.0", 0, false},
		{"v2.0.23", "v1.1", 1, false},
		{"", "V1.1", 0, true},
		{"1.0.23", "", 0, true},
		{"V1.0.23", "一二三", 0, true},
	}

	for _, c := range cases {
		got, err := CompareVersions(c.newVersion, c.oldVersion)
		if c.wantErr {
			if err == nil {
				t.Errorf("CompareVersions(%q, %q) expected error, got nil", c.newVersion, c.oldVersion)
			}
			continue
		}
		if err != nil {
			t.Errorf("CompareVersions(%q, %q) unexpected error: %v", c.newVersion, c.oldVersion, err)
			continue
		}
		if got != c.want {
			t.Errorf("CompareVersions(%q, %q) == %v, want %v", c.newVersion, c.oldVersion, got, c.want)
		}
	}
}
