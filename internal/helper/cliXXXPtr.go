package helper

func CliStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func CliIntPtr(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}

func CliBoolPtr(b bool) *bool {
	return &b
}

