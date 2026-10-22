package storagex

import "testing"

func TestMinioAccessURL(t *testing.T) {
	cases := []struct {
		name   string
		cfg    *MinioConfig
		key    string
		expect string
	}{
		{"http no cdn", &MinioConfig{Endpoint: "127.0.0.1:9000", Bucket: "scans"}, "a/b.png", "http://127.0.0.1:9000/scans/a/b.png"},
		{"https no cdn", &MinioConfig{Endpoint: "127.0.0.1:9000", Bucket: "scans", Secure: true}, "a/b.png", "https://127.0.0.1:9000/scans/a/b.png"},
		{"cdn domain", &MinioConfig{Endpoint: "127.0.0.1:9000", Bucket: "scans", CDNDomain: "cdn.example.com"}, "a/b.png", "https://cdn.example.com/scans/a/b.png"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &MinioProvider{cfg: tc.cfg}
			if got := p.AccessURL(tc.key); got != tc.expect {
				t.Fatalf("AccessURL = %q, want %q", got, tc.expect)
			}
		})
	}
}
