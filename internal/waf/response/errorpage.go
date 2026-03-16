package response

import (
	"fmt"
	"net/http"
)

// ErrorPageConfig configures error page handling.
type ErrorPageConfig struct {
	Enabled bool
	Mode    string // "production" or "development"
}

// GenericErrorPage returns a generic error page HTML for the given status code.
func GenericErrorPage(status int) []byte {
	title := http.StatusText(status)
	if title == "" {
		title = "Error"
	}
	return []byte(fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>%d %s</title></head>
<body>
<h1>%d %s</h1>
<p>The server encountered an error processing your request.</p>
<hr>
<p><small>OpenLoadBalancer</small></p>
</body>
</html>`, status, title, status, title))
}

// IsServerError returns true if the status code is a 5xx error.
func IsServerError(status int) bool {
	return status >= 500 && status < 600
}
