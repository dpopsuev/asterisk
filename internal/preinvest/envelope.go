package preinvest

// Envelope is the execution envelope (launch + failure list).
// For mock skeleton we keep minimal fields; full shape matches examples/pre-investigation-33195-4.21.
type Envelope struct {
	RunID       string `json:"run_id"`
	LaunchUUID  string `json:"launch_uuid"`
	Name        string `json:"name"`
	FailureList []FailureItem `json:"failure_list"`
}

// FailureItem is one failed step (leaf) in the envelope.
type FailureItem struct {
	ID     int    `json:"id"`
	UUID   string `json:"uuid"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Path   string `json:"path"`

	// Enriched fields (populated by rp.FetchEnvelope from TestItemResource).
	CodeRef      string `json:"code_ref,omitempty"`
	Description  string `json:"description,omitempty"`
	ParentID     int    `json:"parent_id,omitempty"`
	IssueType    string `json:"issue_type,omitempty"`
	IssueComment string `json:"issue_comment,omitempty"`
}
