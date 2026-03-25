package proactive

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseCronExpression parses a 5-field cron expression and computes the next
// matching time after `after`. Supports:
//   - * (any value)
//   - specific values (e.g., 30)
//   - ranges (e.g., 1-5)
//   - steps (e.g., */15, 1-30/5)
//
// Fields: minute hour day month weekday
func parseCronExpression(expr string, after time.Time) time.Time {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return after.Add(1 * time.Minute) // fallback
	}

	// Start from the next minute
	t := after.Truncate(time.Minute).Add(1 * time.Minute)

	// Try up to 366 days to find a match
	for i := 0; i < 526000; i++ {
		minute, _ := expandField(fields[0], 0, 59)
		hour, _ := expandField(fields[1], 0, 23)
		dom, _ := expandField(fields[2], 1, 31)
		month, _ := expandField(fields[3], 1, 12)
		dow, _ := expandField(fields[4], 0, 6)

		if matches(minute, t.Minute()) &&
			matches(hour, t.Hour()) &&
			matches(month, int(t.Month())) {
			// For day: check both day-of-month and day-of-week
			// If both day-of-month and day-of-week are restricted, either matches (OR logic).
			// If only one is restricted, it must match.
			domRestricted := fields[2] != "*"
			dowRestricted := fields[4] != "*"

			dayOk := true
			if domRestricted && dowRestricted {
				dayOk = matches(dom, t.Day()) || matches(dow, int(t.Weekday()))
			} else if domRestricted {
				dayOk = matches(dom, t.Day())
			} else if dowRestricted {
				dayOk = matches(dow, int(t.Weekday()))
			}

			if dayOk {
				return t
			}
		}

		t = t.Add(1 * time.Minute)
	}

	// Fallback: return 1 minute from after
	return after.Add(1 * time.Minute)
}

// expandField expands a cron field into a set of matching integers.
func expandField(field string, min, max int) (map[int]bool, error) {
	result := make(map[int]bool)

	for _, part := range strings.Split(field, ",") {
		if strings.Contains(part, "/") {
			// Step expression: */N or start-end/N
			var rangeStr, stepStr string
			slashIdx := strings.Index(part, "/")
			rangeStr = part[:slashIdx]
			stepStr = part[slashIdx+1:]

			step, err := strconv.Atoi(stepStr)
			if err != nil || step <= 0 {
				return nil, fmt.Errorf("invalid step %q", stepStr)
			}

			rangeMin, rangeMax := min, max
			if rangeStr != "*" {
				if strings.Contains(rangeStr, "-") {
					parts := strings.SplitN(rangeStr, "-", 2)
					rm, err := strconv.Atoi(parts[0])
					if err != nil {
						return nil, fmt.Errorf("invalid range start %q", parts[0])
					}
					rMx, err := strconv.Atoi(parts[1])
					if err != nil {
						return nil, fmt.Errorf("invalid range end %q", parts[1])
					}
					rangeMin = rm
					rangeMax = rMx
				} else {
					v, err := strconv.Atoi(rangeStr)
					if err != nil {
						return nil, fmt.Errorf("invalid value %q", rangeStr)
					}
					rangeMin = v
					rangeMax = v
				}
			}

			for v := rangeMin; v <= rangeMax; v += step {
				if v >= min && v <= max {
					result[v] = true
				}
			}
		} else if strings.Contains(part, "-") {
			// Range: start-end
			parts := strings.SplitN(part, "-", 2)
			start, err := strconv.Atoi(parts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid range start %q", parts[0])
			}
			end, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid range end %q", parts[1])
			}
			for v := start; v <= end; v++ {
				if v >= min && v <= max {
					result[v] = true
				}
			}
		} else if part == "*" {
			for v := min; v <= max; v++ {
				result[v] = true
			}
		} else {
			v, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid value %q", part)
			}
			if v >= min && v <= max {
				result[v] = true
			}
		}
	}

	return result, nil
}

func matches(values map[int]bool, v int) bool {
	return values[v]
}
