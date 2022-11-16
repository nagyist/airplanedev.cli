package utils

// UniqueStrings filters out all duplicate strings from a list of strings.
func UniqueStrings(list []string) []string {
	set := map[string]struct{}{}
	for _, s := range list {
		set[s] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for _, s := range list {
		_, ok := set[s]
		if !ok {
			continue
		}
		delete(set, s)

		out = append(out, s)
	}

	return out
}
