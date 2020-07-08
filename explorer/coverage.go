package explorer

// CovStats represents the coverage information captured during runtime.
// It is essentially a pseudonym for a map[string]uint64
// Each key is a unique BlockID and value is the # of times the block was reached.
type CovStats map[string]uint64

// BlockInfo represents the information necessary for calculating the coverage score
// for the block. Currently, it has 2 fields:
// (i) NumNodes : Number of nodes that covered this block
// (ii) Frequency : Total number of times this block was covered
type BlockInfo struct {
	NumNodes int
	Frequency uint64
}

// Objective is the type that encapsulates the type of scoring function that needs to be applied
type Objective int

const (
	UNIQUE Objective = iota
	FREQUENCY
	NODE_FREQUENCY
)

// CalcCoverageScore calculates the coverage score for the coverage
// of 1 single node for a given objective
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

// CoverageUnion merges the coverage information from cov2 into cov1
// cov1 is modified as part of the execution of this function
func CoverageUnion(cov1, cov2 *CovStats) {
	for block, frequency := range *cov2 {
		if v, ok := (*cov1)[block]; ok {
			(*cov1)[block] = v + frequency
		} else {
			(*cov1)[block] = frequency
		}
	}	
}

// CoverageDifference returns the set difference between cov1 and cov2.
// For input coverage A, B the output is the set difference A - B.
func CoverageDifference(cov1, cov2 *CovStats) *CovStats {
	result := make(CovStats)
	for block, _ := range *cov1 {
		if v, ok := (*cov2)[block]; !ok {
			result[block] = v
		}
	}
	return &result
}

// merge_coverage merges the coverage of all nodes into
// 1 convenient struct that can be used to calculate the coverage score
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

// CalcFullCoverageScore calculates the total coverage across all nodes
// Currently, 3 objective functions are supported.
// UNIQUE: Score is calculated as total number of unique blocks covered across all nodes (Union-Count)
// FREQUENCY: Score is calculated as \Sum_{b \in Blocks} \frac{1}{frequency(b)}. Higher score will be for coverage
//            where all the blocks covered have fewer duplicate blocks
// NODE_FREQUENCY: Score is calculated as \Sum_{b \in Blocks} \frac{1}{frequency(b)} * NodeCount(b). Higher score will
//                 be for coverage where more nodes cover a lot of unique blocks without any duplication
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