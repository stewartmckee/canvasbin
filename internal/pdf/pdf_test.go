package pdf

import "testing"

func TestAllowed(t *testing.T) {
	for _, u := range []string{
		"http://127.0.0.1:8080/x", "http://localhost/", "http://192.168.7.130:3002/api/stacks",
		"http://10.0.0.1/", "http://172.17.0.1/", "http://[::1]/", "http://169.254.169.254/latest",
		"http://100.100.1.1/", "file:///etc/passwd", "ftp://example.com/",
	} {
		if allowed(u) {
			t.Errorf("%s should be blocked", u)
		}
	}
	for _, u := range []string{"data:image/png;base64,AAAA", "https://1.1.1.1/"} {
		if !allowed(u) {
			t.Errorf("%s should be allowed", u)
		}
	}
}
