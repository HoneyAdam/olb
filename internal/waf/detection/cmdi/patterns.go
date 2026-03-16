package cmdi

// dangerousCommands maps command names to threat scores.
// Higher scores indicate more dangerous commands.
var dangerousCommands = map[string]int{
	// File operations
	"cat": 40, "ls": 40, "wget": 75, "curl": 75,
	// System info
	"id": 70, "whoami": 70, "uname": 70, "hostname": 60,
	// Network tools (reverse shell risk)
	"nc": 90, "ncat": 90, "netcat": 90,
	// Destructive
	"rm": 80, "chmod": 70, "chown": 70,
	// Credential access
	"passwd": 85, "shadow": 85,
	// Scripting languages
	"python": 85, "python3": 85, "perl": 85, "ruby": 85, "php": 85,
	// Encoding/data tools
	"base64": 75, "xxd": 70, "dd": 70,
	// DNS/network
	"nslookup": 60, "dig": 60, "ping": 50,
	// Remote access
	"telnet": 80, "ssh": 80, "scp": 80,
	// Text processing with code execution
	"awk": 60, "sed": 60, "xargs": 70,
	// Environment
	"env": 60, "export": 50, "printenv": 60,
}

// shellPaths are absolute paths to shell interpreters.
var shellPaths = []string{
	"/bin/sh", "/bin/bash", "/bin/zsh", "/bin/csh", "/bin/ksh",
	"/usr/bin/env", "/usr/bin/python", "/usr/bin/perl",
	"cmd.exe", "powershell", "powershell.exe",
}
