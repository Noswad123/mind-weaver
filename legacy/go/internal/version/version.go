package version

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func String() string {
	return Version
}

func Detailed() string {
	return Version + " (commit " + Commit + ", built " + Date + ")"
}
