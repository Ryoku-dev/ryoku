package host

import "testing"

func TestRPMVercmpUpstreamVectors(t *testing.T) {
	tests := []struct {
		left  string
		right string
		want  int
	}{
		{"1.0", "1.0", 0},
		{"1.0", "2.0", -1},
		{"2.0.1", "2.0", 1},
		{"2.0.1a", "2.0.1", 1},
		{"5.5p1", "5.5p10", -1},
		{"10xyz", "10.1xyz", -1},
		{"xyz10", "xyz10.1", -1},
		{"xyz.4", "8", -1},
		{"6.0.rc1", "6.0", 1},
		{"10b2", "10a1", 1},
		{"1.0a", "1.0aa", -1},
		{"10.0001", "10.1", 0},
		{"10.0001", "10.0039", -1},
		{"2.0", "2_0", 0},
		{"a+", "a_", 0},
		{"+", "_", 0},
		{"1.0~rc1", "1.0", -1},
		{"1.0~rc1", "1.0~rc2", -1},
		{"1.0~rc1~git123", "1.0~rc1", -1},
		{"1.0^", "1.0", 1},
		{"1.0^git1", "1.0^git2", -1},
		{"1.0^git1", "1.01", -1},
		{"1.0^20160101", "1.0.1", -1},
		{"1.0^20160102", "1.0^20160101^git1", 1},
		{"1.0~rc1^git1", "1.0~rc1", 1},
		{"1.0^git1~pre", "1.0^git1", -1},
	}
	for _, tc := range tests {
		if got := rpmvercmp(tc.left, tc.right); got != tc.want {
			t.Errorf("rpmvercmp(%q, %q) = %d, want %d", tc.left, tc.right, got, tc.want)
		}
		if got := rpmvercmp(tc.right, tc.left); got != -tc.want {
			t.Errorf("rpmvercmp(%q, %q) = %d, want %d", tc.right, tc.left, got, -tc.want)
		}
	}
}

func TestRPMEVRComparison(t *testing.T) {
	tests := []struct {
		left  string
		right string
		want  int
	}{
		{"1:1.0-1.fc44", "0:99.0-1.fc44", 1},
		{"1.0-2.fc44", "1.0-1.fc44", 1},
		{"0:1.0-1.fc44", "1.0-1.fc44", 0},
		{"1.0~rc1-1", "1.0-1", -1},
		{"1.0^git1-1", "1.0-1", 1},
	}
	for _, tc := range tests {
		if got := compareRPMEVR(tc.left, tc.right); got != tc.want {
			t.Errorf("compareRPMEVR(%q, %q) = %d, want %d", tc.left, tc.right, got, tc.want)
		}
	}
}
