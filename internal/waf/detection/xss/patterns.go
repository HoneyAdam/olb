package xss

import "strings"

// dangerousTags are HTML tags commonly used in XSS attacks.
// Used by the WAF detection engine to identify XSS payloads in user input.
var dangerousTags = map[string]int{
	"script":   90,
	"iframe":   50,
	"object":   45,
	"embed":    45,
	"applet":   45,
	"form":     30,
	"base":     40,
	"link":     30,
	"meta":     30,
	"svg":      85,
	"math":     40,
	"video":    30,
	"audio":    30,
	"img":      40,
	"body":     40,
	"input":    30,
	"textarea": 30,
	"select":   25,
	"details":  30,
	"marquee":  30,
}

// eventHandlers are HTML event handler attributes — WAF detection signatures.
var eventHandlers = map[string]bool{
	"onload": true, "onerror": true, "onclick": true, "onmouseover": true,
	"onfocus": true, "onblur": true, "onsubmit": true, "onchange": true,
	"onmouseout": true, "onkeydown": true, "onkeyup": true, "onkeypress": true,
	"ondblclick": true, "oncontextmenu": true, "ondrag": true, "ondragend": true,
	"ondragenter": true, "ondragleave": true, "ondragover": true, "ondragstart": true,
	"ondrop": true, "onmousedown": true, "onmouseup": true, "onmousemove": true,
	"onscroll": true, "onwheel": true, "oncopy": true, "oncut": true,
	"onpaste": true, "onabort": true, "oncanplay": true, "oninput": true,
	"oninvalid": true, "onreset": true, "onsearch": true, "ontoggle": true,
	"onanimationend": true, "onanimationiteration": true, "onanimationstart": true,
	"ontransitionend": true, "onpageshow": true, "onpagehide": true,
	"onhashchange": true, "onpopstate": true, "onresize": true,
	"onbeforeunload": true, "onunload": true, "onstorage": true,
	"onmessage": true, "onoffline": true, "ononline": true,
	"onshow": true, "ontouchstart": true, "ontouchend": true, "ontouchmove": true,
	"onpointerdown": true, "onpointerup": true, "onpointermove": true,
	"onafterprint": true, "onbeforeprint": true,
}

// dangerousProtocols are URI protocols commonly abused in XSS — WAF detection signatures.
var dangerousProtocols = map[string]int{
	"javascript": 80,
	"vbscript":   80,
	"data":       60,
	"blob":       40,
}

// domPatterns are JavaScript DOM patterns that indicate XSS — WAF detection signatures.
// These strings are searched for in user input, NOT executed.
var domPatterns = []struct {
	pattern string
	score   int
}{
	{"document.cookie", 60},
	{"innerhtml", 50},
	{"outerhtml", 50},
	{"constructor", 40},
	{".fromcharcode", 55},
}

// isEventHandler checks if an attribute name is an event handler.
func isEventHandler(attr string) bool {
	return eventHandlers[strings.ToLower(attr)]
}
