package jsonaccess

// recursively merge maps
func mergeMaps(maps ...map[string]any) map[string]any {
	merged := make(map[string]any)

	for _, m := range maps {
		for key, value := range m {
			if existing, ok := merged[key]; ok {
				// If both existing and value are maps, merge recursively
				if existingMap, ok1 := existing.(map[string]any); ok1 {
					if valueMap, ok2 := value.(map[string]any); ok2 {
						merged[key] = mergeMaps(existingMap, valueMap)
						continue
					}
				}
			}
			// Otherwise, overwrite or add the new value
			merged[key] = value
		}
	}

	return merged
}
