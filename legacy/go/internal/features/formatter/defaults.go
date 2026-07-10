package formatter

func defaultHubContent() string {
	return "---\ntags: []\n---\n\n## Topics\n## Research\n## Resources\n"
}

var requiredHeaders = []string{
	"## Topics",
	"## Research",
	"## Resources",
}
