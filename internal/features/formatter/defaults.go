package formatter

func defaultHubContent() string {
	return "---\ntags: []\n---\n\n## Todo\n## Topics\n## Research\n## Resources\n"
}

var requiredHeaders = []string{
	"## Todo",
	"## Topics",
	"## Research",
	"## Resources",
}
