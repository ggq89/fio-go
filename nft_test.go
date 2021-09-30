package fio

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestNft(t *testing.T) {
	acc, api, _, err := newApi()
	if err != nil {
		t.Error(err)
		return
	}
	name, domain := word(), word()

	_, err = api.SignPushActions(NewRegDomain(acc.Actor, domain, acc.PubKey))
	if err != nil {
		t.Error(err)
		return
	}
	_, err = api.SignPushActions(MustNewRegAddress(acc.Actor, Address(name+"@"+domain), acc.PubKey))
	if err != nil {
		t.Error(err)
		return
	}
	time.Sleep(3 * time.Second)
	hash1, hash2 := make([]byte, 32), make([]byte, 32)
	_, _ = rand.Read(hash1)
	_, _ = rand.Read(hash2)
	h1 := hex.EncodeToString(hash1)
	h2 := hex.EncodeToString(hash2)

	toAdd := []NftToAdd{
		{
			ChainCode:       "eth",
			ContractAddress: h1[:16],
			TokenId:         h1[16:],
			Url:             "https://github.com/fioprotocol/fio-go",
			Hash:            h1,
			Metadata:        map[string]string{"creator_url": "https://fioprotocol.io"},
			//Metadata: "",
		},
		{
			ChainCode:       "wax",
			ContractAddress: h2[:16],
			TokenId:         h2[16:],
			Url:             "https://github.com/fioprotocol/fio-go",
			Hash:            h2,
			Metadata:        "",
		},
	}
	addr := name + "@" + domain

	act, err := NewAddNft(addr, toAdd, acc.Actor)
	if err != nil {
		t.Error(err)
		return
	}

	_, err = api.SignPushActions(act)
	if err != nil {
		t.Error(err)
		return
	}

	nfts, err := api.GetNftsFioAddress(addr, 0, 100)
	if err != nil {
		t.Error(err)
		return
	}

	if nfts == nil || nfts.Nfts == nil || len(nfts.Nfts) != 2 {
		t.Error("did not get correct count of NFTs in GetNftsFioAddress response")
		return
	}

	nfts, err = api.GetNftsContract("eth", h1[:16], "", 0, 100)
	if err != nil {
		t.Error(err)
		return
	}
	if nfts == nil || nfts.Nfts == nil || len(nfts.Nfts) != 1 {
		t.Error("did not get correct count of NFTs in GetNftsContract response")
		return
	}
	if nfts.Nfts[0].Hash != h1 {
		t.Error("wrong hash returned on GetNftsContract query")
	}

	nfts, err = api.GetNftsHash(h1, 0, 100)
	if err != nil {
		t.Error(err)
		return
	}
	if nfts == nil || nfts.Nfts == nil || len(nfts.Nfts) != 1 {
		t.Error("did not get correct count of NFTs in GetNftsHash response")
		return
	}
	if nfts.Nfts[0].TokenId != h1[16:] {
		t.Error("wrong token id returned on GetNftsContract query")
	}

	act, err = NewRemNft(addr, []NftToDelete{
		{
			ChainCode:       "eth",
			ContractAddress: h1[:16],
			TokenId:         h1[16:],
		},
	},
		acc.Actor,
	)
	if err != nil {
		t.Error(err)
		return
	}
	_, err = api.SignPushActions(act)
	if err != nil {
		t.Error(err)
		return
	}

	nfts, err = api.GetNftsHash(h1, 0, 100)
	if err == nil {
		t.Error("NFT was not deleted")
		return
	}
	nfts, err = api.GetNftsFioAddress(addr, 0, 100)
	if err != nil {
		t.Error(err)
		return
	}

	if nfts == nil || nfts.Nfts == nil || len(nfts.Nfts) != 1 {
		t.Error("did not get correct count of NFTs after RemNft")
		return
	}

	_, err = api.SignPushActions(NewRemAllNft(addr, acc.Actor))
	if err != nil {
		t.Error(err)
		return
	}
	time.Sleep(time.Second)

	remallnfts, err := api.GetNftsFioAddress(addr, 0, 100)
	if err != nil {
		t.Error(err)
		return
	}

	if len(remallnfts.Nfts) != 0 {
		j, _ := json.MarshalIndent(remallnfts, "", "  ")
		fmt.Println(string(j))
		t.Error("failed to RemAllNft, still has NFTs mapped")
	}

}
