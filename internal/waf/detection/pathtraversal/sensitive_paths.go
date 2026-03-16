package pathtraversal

// sensitivePath pairs a file path pattern with a threat score.
type sensitivePath struct {
	path  string
	score int
}

// sensitivePaths contains known sensitive file paths per OS.
// Used by the path traversal detector to identify attempts to access critical files.
var sensitivePaths = []sensitivePath{
	// Unix/Linux
	{"/etc/passwd", 90},
	{"/etc/shadow", 95},
	{"/etc/hosts", 70},
	{"/proc/self", 90},
	{"/proc/version", 80},
	{"/proc/cmdline", 85},

	// Windows
	{"win.ini", 75},
	{"boot.ini", 75},
	{"web.config", 70},

	// Web server config
	{".htaccess", 65},
	{".htpasswd", 80},

	// Application files
	{".env", 75},
	{".git/config", 80},
	{".ssh/", 85},
	{"id_rsa", 90},
	{"authorized_keys", 80},
}
