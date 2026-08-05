package entity

import "time"

type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusProcessed  Status = "processed"
	StatusFailed     Status = "failed"
)

// Params is the filter a roster report is generated against: every individual
// whose m_id is in MIDs AND whose profession_type_id is in ProfessionTypeIDs.
type Params struct {
	MIDs              []int  `json:"m_ids"`
	ProfessionTypeIDs []int  `json:"profession_type_ids"`
	FileName          string `json:"file_name"`
	AccountID         int    `json:"account_id"`
}

type Report struct {
	ID           int64      `json:"id"`
	Status       Status     `json:"status"`
	Params       Params     `json:"params"`
	OutputPath   *string    `json:"output_path,omitempty"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}
