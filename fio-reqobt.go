package fio

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/btcsuite/btcutil"
	"github.com/eoscanada/eos-go/ecc"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"math/rand"
	"time"
)

// ObtContent holds private transaction details for actions such as requesting funds and recording the result
// of a transaction. This should be encrypted and supplied as hex-encoded bytes in the transaction.
type ObtContent struct {
	PayerPublicAddress string `json:"payer_public_address"`
	PayeePublicAddress string `json:"payee_public_address"`
	Amount             string `json:"amount"`
	TokenCode          string `json:"token_code"`
	Status             string `json:"status"`
	ObtId              string `json:"obt_id"`
	Memo               string `json:"memo"`
	Hash               string `json:"hash"`
	OfflineUrl         string `json:"offline_url"`
}

// DecryptContent provides a new populated ObtContent struct given an encrypted content payload
func DecryptContent(to *Account, fromPubKey string, encrypted []byte) (*ObtContent, error) {
	jsonBytes, err := EciesDecrypt(to, fromPubKey, encrypted)
	if err != nil {
		return nil, err
	}
	content := &ObtContent{}
	err = json.Unmarshal(jsonBytes, content)
	if err != nil {
		return nil, err
	}
	return content, nil
}

// Encrypt serializes and encrypts the 'content' field for OBT requests
func (c *ObtContent) Encrypt(from *Account, toPubKey string) (content string, err error) {
	j, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	encrypted, err := EciesEncrypt(from, toPubKey, j)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(encrypted), nil
}

type RecordSend struct {
	FioRequestId    string `json:"fio_request_id"`
	PayerFioAddress string `json:"payer_fio_address"`
	PayeeFioAddress string `json:"payee_fio_address"`
	Content         string `json:"content"`
	MaxFee          uint64 `json:"max_fee"`
	Actor           string `json:"actor"`
	Tpid            string `json:"tpid"`
}

// NewRecordSend builds the action for providing the result of a off-chain transaction
func NewRecordSend(actor eos.AccountName, reqId string, payer string, payee string, content string) *eos.Action {
	return newAction(
		"fio.reqobt", "recordsend", actor,
		RecordSend{
			FioRequestId:    reqId,
			PayerFioAddress: payer,
			PayeeFioAddress: payee,
			Content:         content,
			MaxFee:          Tokens(GetMaxFee("record_send")),
			Actor:           string(actor),
			Tpid:            globalTpid,
		},
	)
}

// FundsReq is a request sent from one user to another requesting funds
type FundsReq struct {
	PayerFioAddress string `json:"payer_fio_address"`
	PayeeFioAddress string `json:"payee_fio_address"`
	Content         string `json:"content"`
	MaxFee          uint64 `json:"max_fee"`
	Actor           string `json:"actor"`
	Tpid            string `json:"tpid"`
}

// NewFundsReq builds the action for providing the result of a off-chain transaction
func NewFundsReq(actor eos.AccountName, payer string, payee string, content string) *eos.Action {
	return newAction(
		"fio.reqobt", "newfundsreq", actor,
		FundsReq{
			PayerFioAddress: payer,
			PayeeFioAddress: payee,
			Content:         content,
			MaxFee:          Tokens(GetMaxFee("new_funds_request")),
			Actor:           string(actor),
			Tpid:            globalTpid,
		},
	)
}

// RejectFndReq is a response to a user, denying their request for funds.
type RejectFndReq struct {
	FioRequestId string `json:"fio_request_id"`
	MaxFee       uint64 `json:"max_fee"`
	Actor        string `json:"actor"`
	Tpid         string `json:"tpid"`
}

// NewRejectFndReq builds the action to reject a request
func NewRejectFndReq(actor eos.AccountName, requestId string) *eos.Action {
	return newAction(
		"fio.reqobt", "rejectfndreq", actor,
		RejectFndReq{
			FioRequestId: requestId,
			MaxFee:       Tokens(GetMaxFee("reject_funds_request")),
			Actor:        string(actor),
			Tpid:         globalTpid,
		},
	)
}

// EciesEncrypt implements the encryption format used in the content field of OBT requests. A DH shared secret is
// created using ECIES which derives a key based on the curves of the public and private keys.
// This secret is hashed using sha512, and the first 32 bytes of the hash is used to encrypt the message using
// AES-256 cbc, and the second half is used to create an outer sha256 hmac. A 16 byte IV is prepended to the
// output, resulting in the message format of: IV + Ciphertext + HMAC
// See https://github.com/fioprotocol/fiojs/blob/master/docs/message_encryption.md for more information.
func EciesEncrypt(sender *Account, recipentPub string, plainText []byte) (content []byte, err error) {
	var buffer bytes.Buffer

	// Get the shared-secret
	_, secretHash, e := EciesSecret(sender, recipentPub)
	if e != nil {
		return nil, e
	}

	// Generate IV
	iv := make([]byte, 16)
	rand.Seed(time.Now().UnixNano())
	_, e = rand.Read(iv)
	if e != nil {
		return nil, e
	}
	buffer.Write(iv)

	// setup AES CBC for encryption
	block, e := aes.NewCipher(secretHash[:32])
	if e != nil {
		return nil, e
	}
	cbc := cipher.NewCBCEncrypter(block, iv)

	// create pkcs#7 padding
	pad := func() []byte {
		padLen := block.BlockSize() - (len(plainText) % block.BlockSize())
		if padLen == 0 {
			padLen = block.BlockSize()
		}
		pad := make([]byte, 0)
		for i := 0; i < padLen; i++ {
			pad = append(pad, byte(padLen))
		}
		return pad
	}()

	// encrypt the plaintext
	cipherText := make([]byte, len(plainText)+len(pad))
	cbc.CryptBlocks(cipherText, append(plainText, pad...))
	buffer.Write(cipherText)

	// Sign the message using sha256 hmac
	signer := hmac.New(sha256.New, secretHash[32:])
	_, e = signer.Write(buffer.Bytes())
	if e != nil {
		return nil, e
	}
	signature := signer.Sum(nil)
	buffer.Write(signature)

	return buffer.Bytes(), nil
}

// EciesDecrypt is the inverse of EciesEncrypt, using the recipient's private key and sender's public instead.
func EciesDecrypt(recipient *Account, senderPub string, message []byte) (decrypted []byte, err error) {
	const (
		ivLen  = 16
		sigLen = 32
	)

	// Get the shared-secret
	_, secretHash, e := EciesSecret(recipient, senderPub)
	if e != nil {
		return nil, e
	}

	// check the signature
	verifier := hmac.New(sha256.New, secretHash[32:])
	_, err = verifier.Write(message[:len(message)-sigLen])
	if err != nil {
		return nil, err
	}
	verified := verifier.Sum(nil)
	if hex.EncodeToString(message[len(message)-sigLen:]) != hex.EncodeToString(verified) {
		return nil,
			errors.New(
				fmt.Sprintf("hmac signature %s is invalid, expected %s",
					hex.EncodeToString(verified),
					hex.EncodeToString(message[len(message)-sigLen:]),
				),
			)
	}

	// decrypt the message
	block, err := aes.NewCipher(secretHash[:32])
	if err != nil {
		return nil, err
	}
	cbc := cipher.NewCBCDecrypter(block, message[:ivLen])
	plainText := make([]byte, len(message[ivLen:len(message)-sigLen]))
	cbc.CryptBlocks(plainText, message[ivLen:len(message)-sigLen])

	// strip padding and done.
	padLen := int(plainText[len(plainText)-1])
	if padLen >= len(plainText) {
		return nil, errors.New("invalid padding in message")
	}
	return plainText[:len(plainText)-padLen], nil
}

// EciesSecret derives the ecies pre-shared key from a private and public key.
// The 'secret' returned is the actual secret, the 'hash' returned is what is actually used
// in the OBT implementation, allowing the secret to be stretched into two keys, one for
// encryption and one for message authentication.
func EciesSecret(private *Account, public string) (secret []byte, hash []byte, err error) {
	// convert key to ecies private key type
	wif, err := btcutil.DecodeWIF(private.KeyBag.Keys[0].String())
	if err != nil {
		return nil, nil, err
	}
	priv := ecies.ImportECDSA(wif.PrivKey.ToECDSA())

	// convert public key string into an ecies public key struct
	eosPub, err := ecc.NewPublicKey(`EOS` + public[3:])
	if err != nil {
		return nil, nil, err
	}
	epk, err := eosPub.Key()
	if err != nil {
		return nil, nil, err
	}
	pub := ecies.ImportECDSAPublic(epk.ToECDSA())

	// derive the shared secret and hash it
	sharedKey, err := priv.GenerateShared(pub, 32, 0)
	if err != nil {
		return nil, nil, err
	}
	sh := sha512.New()
	_, err = sh.Write(sharedKey)
	if err != nil {
		return nil, nil, err
	}
	return sharedKey, sh.Sum(nil), nil
}
