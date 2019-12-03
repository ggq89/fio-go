package fio

import "github.com/eoscanada/eos-go"

type PayTpidRewards struct {
	Actor eos.AccountName `json:"actor"`
}

func NewPayTpidRewards(actor eos.AccountName) *eos.Action {
	return newAction(
		eos.AccountName("fio.treasury"), "tpidclaim", actor,
		PayTpidRewards{Actor: actor},
	)
}

type BpClaim struct {
	FioAddress string          `json:"fio_address"`
	Actor      eos.AccountName `json:"actor"`
}

func NewBpClaim(fioAddress string, actor eos.AccountName) *eos.Action {
	return newAction(
		eos.AccountName("fio.treasury"), "bpclaim", actor,
		BpClaim{
			FioAddress: fioAddress,
			Actor:      actor,
		},
	)
}