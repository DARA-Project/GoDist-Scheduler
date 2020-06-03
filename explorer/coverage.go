package explorer

type CovStats map[string]uint64

type Objective int

type BlockInfo struct {
	NumNodes int
	Frequency uint64
}

const (
	UNIQUE Objective = iota
	FREQUENCY
	NODE_FREQUENCY
)

func CalcCoverageScore(coverage *CovStats, obj Objective) float64 {
	var score float64
	switch obj {
	case UNIQUE:
		score = float64(len(*coverage))
	case FREQUENCY:
	case NODE_FREQUENCY:
		for _, frequency := range *coverage {
			score += 1.0 / float64(frequency)
		}
	}
	return score
}

func merge_coverage(allCoverage map[int]*CovStats) map[string]*BlockInfo {
	merged_coverage := make(map[string]*BlockInfo)
	for _, stats := range allCoverage {
		for block, frequency := range *stats {
			if binfo, ok := merged_coverage[block]; ok {
				binfo.Frequency += frequency
				binfo.NumNodes += 1
				merged_coverage[block] = binfo
			} else {
				merged_coverage[block] = &BlockInfo{1, frequency}
			}
		}
	}
	return merged_coverage
}

func CalcFullCoverageScore(allCoverage map[int]*CovStats, obj Objective) float64 {
	flattened_coverage := merge_coverage(allCoverage)
	var score float64
	switch obj {
	case UNIQUE:
		score = float64(len(flattened_coverage))
	case FREQUENCY:
		for _, info := range flattened_coverage {
			score += 1.0 / float64(info.Frequency)
		}
	case NODE_FREQUENCY:
		for _, info := range flattened_coverage {
			score += 1.0 / float64(info.Frequency) * float64(info.NumNodes)
		}
	}
	return score
}