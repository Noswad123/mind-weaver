package parser

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	explicitWeightRe = regexp.MustCompile(`(?i)\b(?:w|weight):\s*([0-9]+(?:\.[0-9]+)?)\b`)
	priorityTokenRe  = regexp.MustCompile(`(?i)\bp:?([1-5])\b`)
	energyTokenRe    = regexp.MustCompile(`(?i)\be:(xsm|xs|x-small|small|s|medium|m|large|l|xl|x-large)\b`)
	dueDateTokenRe   = regexp.MustCompile(`(?i)\bdue:\s*(\d{4}-\d{2}-\d{2})\b`)
	startDateTokenRe = regexp.MustCompile(`(?i)\bstart:\s*(\d{4}-\d{2}-\d{2})\b`)
)

func DeriveTodoWeight(taskText string) float64 {
	return DeriveTodoWeightWithDefaults(taskText, "p3", "medium")
}

func DeriveTodoWeightWithDefaults(taskText string, defaultPriority string, defaultEnergy string) float64 {
	return deriveTodoWeightWithDefaultsAt(taskText, defaultPriority, defaultEnergy, time.Now())
}

func deriveTodoWeightWithDefaultsAt(taskText string, defaultPriority string, defaultEnergy string, now time.Time) float64 {
	if w := parseExplicitWeight(taskText); w > 0 {
		return w
	}

	priorityWeight := parsePriorityWeight(taskText, defaultPriority)
	energyMultiplier := parseEnergyMultiplier(taskText, defaultEnergy)
	dueMultiplier := parseDueDateMultiplier(taskText, now)
	startMultiplier := parseStartDateMultiplier(taskText, now)

	weight := priorityWeight * energyMultiplier * dueMultiplier * startMultiplier
	if weight <= 0 {
		return 1
	}
	return weight
}

func parseExplicitWeight(taskText string) float64 {
	m := explicitWeightRe.FindStringSubmatch(taskText)
	if len(m) < 2 {
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(m[1]), 64)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

func parsePriorityWeight(taskText string, defaultPriority string) float64 {
	m := priorityTokenRe.FindStringSubmatch(taskText)
	if len(m) >= 2 {
		return priorityWeightForToken("p" + strings.TrimSpace(strings.ToLower(m[1])))
	}

	if normalized, ok := normalizePriorityToken(defaultPriority); ok {
		return priorityWeightForToken(normalized)
	}

	return 1
}

func normalizePriorityToken(raw string) (string, bool) {
	v := strings.TrimSpace(strings.ToLower(raw))
	v = strings.TrimPrefix(v, "p")
	if v == "" {
		return "", false
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 5 {
		return "", false
	}
	return "p" + strconv.Itoa(n), true
}

func priorityWeightForToken(priority string) float64 {
	switch priority {
	case "p1":
		return 2
	case "p2":
		return 1.5
	case "p3":
		return 1
	case "p4":
		return 0.75
	case "p5":
		return 0.5
	default:
		return 1
	}
}

func parseEnergyMultiplier(taskText string, defaultEnergy string) float64 {
	m := energyTokenRe.FindStringSubmatch(taskText)
	if len(m) >= 2 {
		if token, ok := normalizeEnergyToken(m[1]); ok {
			return energyMultiplierForToken(token)
		}
	}

	if token, ok := normalizeEnergyToken(defaultEnergy); ok {
		return energyMultiplierForToken(token)
	}

	return 1
}

func normalizeEnergyToken(raw string) (string, bool) {
	v := strings.TrimSpace(strings.ToLower(raw))
	switch v {
	case "xsm", "xs", "x-small":
		return "x-small", true
	case "small", "s":
		return "small", true
	case "m", "medium":
		return "medium", true
	case "large", "l":
		return "large", true
	case "xl", "x-large":
		return "x-large", true
	default:
		return "", false
	}
}

func energyMultiplierForToken(energy string) float64 {
	switch energy {
	case "x-large":
		return 1.30
	case "large":
		return 1.15
	case "small":
		return 0.85
	case "x-small":
		return 0.70
	case "medium":
		return 1
	default:
		return 1
	}
}

func parseDueDateMultiplier(taskText string, now time.Time) float64 {
	due, ok := parseDateToken(taskText, dueDateTokenRe, now.Location())
	if !ok {
		return 1
	}

	today := startOfDay(now)
	days := int(due.Sub(today).Hours() / 24)
	switch {
	case days < 0:
		return 1.35
	case days == 0:
		return 1.25
	case days <= 3:
		return 1.15
	case days <= 7:
		return 1.08
	case days <= 14:
		return 1.03
	default:
		return 1
	}
}

func parseStartDateMultiplier(taskText string, now time.Time) float64 {
	start, ok := parseDateToken(taskText, startDateTokenRe, now.Location())
	if !ok {
		return 1
	}

	today := startOfDay(now)
	days := int(start.Sub(today).Hours() / 24)
	if days <= 0 {
		return 1
	}
	switch {
	case days <= 7:
		return 0.85
	case days <= 14:
		return 0.70
	default:
		return 0.55
	}
}

func parseDateToken(taskText string, re *regexp.Regexp, loc *time.Location) (time.Time, bool) {
	m := re.FindStringSubmatch(taskText)
	if len(m) < 2 {
		return time.Time{}, false
	}
	d, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(m[1]), loc)
	if err != nil {
		return time.Time{}, false
	}
	return startOfDay(d), true
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
