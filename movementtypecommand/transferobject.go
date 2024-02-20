package movementtypecommand

type RegisterNewMovementTypeTransferObject struct {
	Action          string   `json:"action" binding:"required"`
	AccountId       string   `json:"accountId" binding:"required,uuid"`
	SourceAccountId *string  `json:"sourceAccountId" binding:"omitempty,uuid"`
	Description     string   `json:"description" binding:"required"`
	Notes           *string  `json:"notes"`
	TagIds          []string `json:"tagIds" binding:"required"`
}
