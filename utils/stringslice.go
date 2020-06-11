package utils

func CompareStringSliceElements(lh []string, rh []string) bool {
	if len(lh) != len(rh) {
		return false
	}

	lhElements := make(map[string]bool)
	for _, val := range lh {
		lhElements[val] = true
	}

	for _, val := range rh {
		if !lhElements[val] {
			return false
		}
	}

	return true
}
