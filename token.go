package fio

import (
	"github.com/fioprotocol/fio-go/eos"
	"github.com/shopspring/decimal"
)

const FioSymbol = "ᵮ"

// Tokens is a convenience function for converting from a float for human readability.
// Example 1 FIO Token: Tokens(1.0) == uint64(1000000000)
func Tokens(tokens float64) uint64 {
	return uint64(decimal.NewFromFloat(tokens).Mul(decimal.NewFromInt(1000000000)).IntPart())
}

// TransferTokensPubKey is used to send FIO tokens to a public key
type TransferTokensPubKey struct {
	PayeePublicKey string          `json:"payee_public_key"`
	Amount         uint64          `json:"amount"`
	MaxFee         uint64          `json:"max_fee"`
	Actor          eos.AccountName `json:"actor"`
	Tpid           string          `json:"tpid"`
}

// NewTransferTokensPubKey builds an eos.Action for sending FIO tokens
func NewTransferTokensPubKey(actor eos.AccountName, recipientPubKey string, amount uint64) *Action {
	return NewAction(
		"fio.token", "trnsfiopubky", actor,
		TransferTokensPubKey{
			PayeePublicKey: recipientPubKey,
			Amount:         amount,
			MaxFee:         Tokens(GetMaxFee(FeeTransferTokensPubKey)),
			Actor:          actor,
			Tpid:           CurrentTpid(),
		},
	)
}

// Transfer is a privileged call, and not normally used for sending tokens, use TransferTokensPubKey instead
type Transfer struct {
	From     eos.AccountName `json:"from"`
	To       eos.AccountName `json:"to"`
	Quantity eos.Asset       `json:"quantity"`
	Memo     string          `json:"memo"`
}

// NewTransfer is unlikely to be called, this is a privileged action
//
// deprecated: internal action, user cannot call.
func NewTransfer(actor eos.AccountName, recipient eos.AccountName, amount uint64) *Action {
	return NewAction(
		eos.AccountName("fio.token"), "transfer", actor,
		Transfer{
			From: actor,
			To:   recipient,
			Quantity: eos.Asset{
				Amount: eos.Int64(amount),
				Symbol: eos.Symbol{
					Precision: 9,
					Symbol:    "FIO",
				},
			},
		},
	)
}

// GetBalance gets an account's balance
func (api *API) GetBalance(account eos.AccountName) (float64, error) {
	a, err := api.GetCurrencyBalance(account, "FIO", eos.AccountName("fio.token"))
	if err != nil {
		return 0.0, err
	}
	if len(a) > 0 {
		if a[0].Amount > 0 {
			return float64(a[0].Amount) / 1000000000.0, nil
		}
	}
	return 0.0, nil
}

type GetFioBalanceResp struct {
	Balance   uint64 `json:"balance"`
	Available uint64 `json:"available"`
}

type getFioBalanceReq struct {
	FioPublicKey string `json:"fio_public_key"`
}

// GetFioBalance is the preferred way to get an account's balance, it will include the number of available
// tokens in the case that an account holds locked/staked tokens.
func (api *API) GetFioBalance(pubkey string) (fiobalance *GetFioBalanceResp, err error) {
	err = api.call("chain", "get_fio_balance", &getFioBalanceReq{FioPublicKey: pubkey}, &fiobalance)
	return fiobalance, err
}
