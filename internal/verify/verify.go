package verify

type DatabaseState struct {
	RowsInserted   int      `json:"rowsInserted"`
	RowsUpdated    int      `json:"rowsUpdated"`
	RowsDeleted    int      `json:"rowsDeleted"`
	MatchedTables  []string `json:"matchedTables"`
	SnapshotBefore int      `json:"snapshotBefore"`
	SnapshotAfter  int      `json:"snapshotAfter"`
}

func Empty() DatabaseState {
	return DatabaseState{MatchedTables: []string{}}
}
