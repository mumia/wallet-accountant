package movementtype

type RegisterNewMovementTypeTransferObject struct {
	Type            string   `json:"type" binding:"required"`
	AccountId       string   `json:"accountId" binding:"required,uuid"`
	SourceAccountId *string  `json:"sourceAccountId"`
	Description     string   `json:"description" binding:"required"`
	Notes           *string  `json:"notes"`
	TagIds          []string `json:"tags" binding:"required"`
}
