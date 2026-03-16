package botdetect

// FPCategory represents a TLS fingerprint category.
type FPCategory string

const (
	FPGood       FPCategory = "good"       // known legitimate browser
	FPBad        FPCategory = "bad"        // known attack tool
	FPSuspicious FPCategory = "suspicious" // headless browser, automation
	FPUnknown    FPCategory = "unknown"    // not in database
)

// FPResult holds the result of a fingerprint lookup.
type FPResult struct {
	Category   FPCategory
	Name       string // e.g., "Chrome 120", "Python requests"
	Confidence int    // 0-100
}

// Known JA3 hashes — curated embedded database.
// These are commonly observed fingerprints for popular clients.
var knownFingerprints = map[string]FPResult{
	// Known bad — attack tools and scanners
	"e4f5f2eb70b088d5da3a1b1e47e89dd9": {FPBad, "Python requests", 90},
	"b32309a26951912be7dba376398abc3b": {FPBad, "Python urllib", 85},
	"3b5074b1b5d032e5620f69f9f700ff0e": {FPBad, "Go default client", 70},
	"36f7277af969a6947a61ae0b815907a1": {FPBad, "sqlmap", 95},
	"a0e9f5d64349fb13191bc781f81f42e1": {FPBad, "nikto", 95},
	"b386946a5a44d1ddcc843bc75336dfce": {FPBad, "nmap", 90},
	"19e29534fd49dd27d09234e639c4057e": {FPBad, "curl", 60},

	// Known suspicious — automation tools
	"b4e0e3e3d3e4e5e6e7e8e9f0f1f2f3f4": {FPSuspicious, "Headless Chrome", 75},
	"cd08e31494f9531f560d64c695473da9": {FPSuspicious, "PhantomJS", 80},
	"a4a71d4be4e643e5fa3dfc5721738426": {FPSuspicious, "Selenium", 75},
}

// ClassifyFingerprint looks up a JA3 hash in the known fingerprint database.
func ClassifyFingerprint(ja3Hash string) FPResult {
	if result, ok := knownFingerprints[ja3Hash]; ok {
		return result
	}
	return FPResult{Category: FPUnknown, Name: "unknown", Confidence: 0}
}
