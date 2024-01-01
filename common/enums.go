package common

type AccountType string

const (
	Checking AccountType = "checking"
	Savings  AccountType = "savings"
)

type MovementAction string

const (
	Debit  MovementAction = "debit"
	Credit MovementAction = "credit"
)

func MovementActionBuilder(stringMovementAction string) MovementAction {
	return MovementAction(stringMovementAction)
}
