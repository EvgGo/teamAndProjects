package mapper

import (
	"fmt"
	"strconv"
	"strings"
)

func NormalizeProjectSkillIDs(raw []string, max int) ([]int, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	seen := make(map[int]struct{}, len(raw))
	out := make([]int, 0, len(raw))

	for _, item := range raw {
		v := strings.TrimSpace(item)
		if v == "" {
			return nil, fmt.Errorf("skill_ids must not contain empty values")
		}

		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid skill_id: %q", v)
		}

		id := int(n)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}

	if len(out) > max {
		return nil, fmt.Errorf("maximum %d skill_ids allowed", max)
	}

	return out, nil
}
