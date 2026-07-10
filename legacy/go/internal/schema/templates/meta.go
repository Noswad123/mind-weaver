package templates

func ReadDomainFromContent(content string) string {
	domains := readDomains(content)
	if len(domains) == 0 {
		return ""
	}
	return domains[0]
}
