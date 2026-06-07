package coverage

type Coverage struct {
	PageCoverage     float64 `json:"pageCoverage"`
	WorkflowCoverage float64 `json:"workflowCoverage"`
	APICoverage      float64 `json:"apiCoverage"`
	FormCoverage     float64 `json:"formCoverage"`
	TotalPages       int     `json:"totalPages"`
	TestedPages      int     `json:"testedPages"`
	TotalWorkflows   int     `json:"totalWorkflows"`
	TestedWorkflows  int     `json:"testedWorkflows"`
	TotalAPIs        int     `json:"totalApis"`
	TestedAPIs       int     `json:"testedApis"`
	TotalForms       int     `json:"totalForms"`
	TestedForms      int     `json:"testedForms"`
}

func Calculate(totalPages, testedPages, totalWorkflows, testedWorkflows, totalAPIs, testedAPIs, totalForms, testedForms int) Coverage {
	return Coverage{
		PageCoverage:     pct(testedPages, totalPages),
		WorkflowCoverage: pct(testedWorkflows, totalWorkflows),
		APICoverage:      pct(testedAPIs, totalAPIs),
		FormCoverage:     pct(testedForms, totalForms),
		TotalPages:       totalPages,
		TestedPages:      testedPages,
		TotalWorkflows:   totalWorkflows,
		TestedWorkflows:  testedWorkflows,
		TotalAPIs:        totalAPIs,
		TestedAPIs:       testedAPIs,
		TotalForms:       totalForms,
		TestedForms:      testedForms,
	}
}

func pct(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}
