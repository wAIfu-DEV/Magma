package lineidx

func GetLineIdx(bytes []byte) []int {
	indexes := make([]int, 0, 8)

	indexes = append(indexes, -1)
	for i, b := range bytes {
		if b == '\n' {
			indexes = append(indexes, i)
		}
	}
	indexes = append(indexes, len(bytes))
	return indexes
}
