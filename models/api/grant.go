package api

// GiveFreeGrantsRequest gives the same quotas to one or several emails (an email already given is overwritten).
type GiveFreeGrantsRequest struct {
	Emails []string       `json:"emails" binding:"required"`
	Quotas map[string]int `json:"quotas" binding:"required"`
	Note   string         `json:"note"`
}

// UpdateFreeGrantRequest replaces the quotas and the note of one email.
type UpdateFreeGrantRequest struct {
	Quotas map[string]int `json:"quotas" binding:"required"`
	Note   string         `json:"note"`
}
