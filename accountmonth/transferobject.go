package accountmonth

import (
	"time"
)

type RegisterNewAccountMovementTransferObject struct {
	AccountId      string    `json:"accountId" binding:"required,uuid"`
	MovementTypeId string    `json:"movementTypeId" binding:"required,uuid"`
	Amount         float64   `json:"amount" binding:"required"`
	Date           time.Time `json:"date" binding:"required"` // format needs to be 2018-08-26T00:00:00Z
}

type EndAccountMonthTransferObject struct {
	AccountId  string     `json:"accountId" binding:"required,uuid"`
	EndBalance float64    `json:"endBalance" binding:"required"`
	Month      time.Month `json:"month" binding:"required"`
	Year       uint       `json:"year" binding:"required"`
}
