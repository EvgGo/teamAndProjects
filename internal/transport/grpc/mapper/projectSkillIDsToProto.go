package mapper

import "strconv"

func ProjectSkillIDsToProto(ids []int) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, strconv.FormatInt(int64(id), 10))
	}
	return out
}
