package ledgercommand

import (
	"time"
)

type RegisterNewAccountMovementTransferObject struct {
	AccountId       string    `json:"accountId" binding:"required,uuid"`
	MovementTypeId  *string   `json:"movementTypeId" binding:"omitempty,uuid"`
	Amount          int64     `json:"amount" binding:"required"`
	Date            time.Time `json:"date" binding:"required"` // format needs to be 2018-08-26T00:00:00Z
	Action          string    `json:"action" binding:"required"`
	SourceAccountId *string   `json:"sourceAccountId" binding:"omitempty,uuid"`
	Description     string    `json:"description" binding:"required"`
	Notes           *string   `json:"notes"`
	TagIds          []string  `json:"tagIds" binding:"required"`
}

type EndAccountMonthTransferObject struct {
	AccountId  string     `json:"accountId" binding:"required,uuid"`
	EndBalance *int64     `json:"endBalance" binding:"required,number"`
	Month      time.Month `json:"month" binding:"required"`
	Year       uint       `json:"year" binding:"required"`
}
