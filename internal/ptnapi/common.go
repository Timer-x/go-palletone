package ptnapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/palletone/go-palletone/common"
	"github.com/palletone/go-palletone/common/log"
	"github.com/palletone/go-palletone/core"
	"github.com/palletone/go-palletone/core/accounts"
	"github.com/palletone/go-palletone/core/accounts/keystore"
	"github.com/palletone/go-palletone/dag/dagconfig"
	"github.com/palletone/go-palletone/dag/modules"
	"github.com/palletone/go-palletone/tokenengine"
	"github.com/shopspring/decimal"
)

func unlockKS(b Backend, addr common.Address, password string, timeout *Int) error {
	ks := b.GetKeyStore()
	if ks.IsUnlock(addr) {
		return nil
	}
	if password != "" {
		if timeout == nil {
			return ks.Unlock(accounts.Account{Address: addr}, password)
		} else {
			d := time.Duration(timeout.Uint32()) * time.Second
			return ks.TimedUnlock(accounts.Account{Address: addr}, password, d)
		}
	}
	return nil
}

func parseAddressStr(addr string, ks *keystore.KeyStore, password string) (common.Address, error) {
	addrArray := strings.Split(addr, ":")
	addrString := addrArray[0]
	if len(addrArray) == 2 { //HD wallet address format
		toAccountIndex, err := strconv.Atoi(addrArray[1])
		if err != nil {
			return common.Address{}, errors.New("invalid addrString address format")
		}
		hdBaseAddr, err := common.StringToAddress(addrArray[0])
		if err != nil {
			return common.Address{}, errors.New("invalid addrString address format")
		}
		var toAccount accounts.Account
		if ks.IsUnlock(hdBaseAddr) {
			toAccount, err = ks.GetHdAccount(accounts.Account{Address: hdBaseAddr}, uint32(toAccountIndex))
		} else {
			toAccount, err = ks.GetHdAccountWithPassphrase(accounts.Account{Address: hdBaseAddr}, password, uint32(toAccountIndex))
		}
		if err != nil {
			return common.Address{}, errors.New("GetHdAccountWithPassphrase error:" + err.Error())
		}
		addrString = toAccount.Address.String()
	}
	return common.StringToAddress(addrString)
}

func buildRawTransferTx(b Backend, tokenId, fromStr, toStr string, amount, gasFee decimal.Decimal, password string) (
	*modules.Transaction, []*modules.UtxoWithOutPoint, error) {
	//参数检查
	tokenAsset, err := modules.StringToAsset(tokenId)
	if err != nil {
		return nil, nil, err
	}
	deci := tokenAsset.GetDecimal()
	dnumber_slice := strings.Split(fmt.Sprintf("%v", amount), ".")
	if len(dnumber_slice) > 1 && len(dnumber_slice[1]) > int(deci) {
		return nil, nil, fmt.Errorf("Asset Deimal is error")
	}

	if !gasFee.IsPositive() && b.EnableGasFee() {
		return nil, nil, fmt.Errorf("fee is ZERO ")
	}
	//
	fromAddr, err := parseAddressStr(fromStr, b.GetKeyStore(), password)
	if err != nil {
		fmt.Println(err.Error())
		return nil, nil, err
	}
	//from := fromAddr.String()
	toAddr, err := parseAddressStr(toStr, b.GetKeyStore(), password)
	if err != nil {
		fmt.Println(err.Error())
		return nil, nil, err
	}
	//to:=toAddr.String()
	ptnAmount := uint64(0)
	gasToken := dagconfig.DagConfig.GasToken
	gasAsset := dagconfig.DagConfig.GetGasToken()
	if tokenId == gasToken {
		ptnAmount = gasAsset.Uint64Amount(amount)
	}
	tx := modules.NewTransaction([]*modules.Message{})
	//构造转移PTN的Message0
	//var dbUtxos map[modules.OutPoint]*modules.Utxo
	//var reqTxMapping map[common.Hash]common.Hash
	//dbUtxos, reqTxMapping, err = b.Dag().GetAddrUtxoAndReqMapping(fromAddr, nil)
	//
	//if err != nil {
	//	return nil, nil, fmt.Errorf("GetAddrRawUtxos utxo err:%s", err.Error())
	//}
	//log.DebugDynamic(func() string {
	//	utxoKeys := ""
	//	for o := range dbUtxos {
	//		utxoKeys += o.String() + ";"
	//	}
	//	mapping := ""
	//	for req, tx := range reqTxMapping {
	//		mapping += req.String() + ":" + tx.String() + ";"
	//	}
	//	return "db utxo outpoints:" + utxoKeys + " req:tx mapping :" + mapping
	//})
	//poolTxs, err := b.GetUnpackedTxsByAddr(from)
	//if err != nil {
	//	return nil, nil, fmt.Errorf("GetUnpackedTxsByAddr err:%s", err.Error())
	//}
	//log.DebugDynamic(func() string {
	//	txHashs := ""
	//	for _, tx := range poolTxs {
	//		txHashs += "[tx:" + tx.Tx.Hash().String() + "-req:" + tx.Tx.RequestHash().String() + "];"
	//	}
	//	return "txpool unpacked tx:" + txHashs
	//})
	//utxosPTN, err := SelectUtxoFromDagAndPool(dbUtxos, reqTxMapping, poolTxs, from, gasToken)
	usedUtxo1 := []*modules.UtxoWithOutPoint{}
	if b.EnableGasFee() {
		utxosPTN, err := b.GetPoolAddrUtxos(fromAddr, gasAsset.ToAsset())
		if err != nil {
			return nil, nil, fmt.Errorf("SelectUtxoFromDagAndPool utxo err:%s", err.Error())
		}
		feeAmount := gasAsset.Uint64Amount(gasFee)
		var pay1 *modules.PaymentPayload
		pay1, usedUtxo1, err = createPayment(fromAddr, toAddr, ptnAmount, feeAmount, utxosPTN)
		if err != nil {
			return nil, nil, err
		}
		tx.AddMessage(modules.NewMessage(modules.APP_PAYMENT, pay1))
		if tokenId == gasToken {
			return tx, usedUtxo1, nil
		}
	}
	log.Debugf("gas token[%s], transfer token[%s], start build payment1", gasToken, tokenId)
	//构造转移Token的Message1
	//utxosToken, err := SelectUtxoFromDagAndPool(dbUtxos, reqTxMapping, poolTxs, from, tokenId)
	utxosToken, err := b.GetPoolAddrUtxos(fromAddr, tokenAsset)
	if err != nil {
		return nil, nil, fmt.Errorf("SelectUtxoFromDagAndPool token utxo err:%s", err.Error())
	}
	tokenAmount := tokenAsset.Uint64Amount(amount)
	pay2, usedUtxo2, err := createPayment(fromAddr, toAddr, tokenAmount, 0, utxosToken)
	if err != nil {
		return nil, nil, err
	}
	tx.AddMessage(modules.NewMessage(modules.APP_PAYMENT, pay2))
	usedUtxo1 = append(usedUtxo1, usedUtxo2...)

	return tx, usedUtxo1, nil
}

func createPayment(fromAddr, toAddr common.Address, amountToken uint64, feePTN uint64,
	utxosPTN map[modules.OutPoint]*modules.Utxo) (*modules.PaymentPayload, []*modules.UtxoWithOutPoint, error) {

	if len(utxosPTN) == 0 {
		log.Errorf("No PTN Utxo or No Token Utxo for %s", fromAddr.String())
		return nil, nil, fmt.Errorf("No Utxo found for %s", fromAddr.String())
	}

	//PTN
	utxoPTNView, asset := convertUtxoMap2Utxos(utxosPTN)

	utxosPTNTaken, change, err := core.Select_utxo_Greedy(utxoPTNView, amountToken+feePTN)
	if err != nil {
		return nil, nil, fmt.Errorf("createPayment Select_utxo_Greedy utxo err:%s", err.Error())
	}
	usedUtxo := []*modules.UtxoWithOutPoint{}
	//ptn payment
	payPTN := &modules.PaymentPayload{}
	//ptn inputs
	for _, u := range utxosPTNTaken {
		utxo := u.(*modules.UtxoWithOutPoint)
		usedUtxo = append(usedUtxo, utxo)
		prevOut := &utxo.OutPoint // modules.NewOutPoint(txHash, utxo.MessageIndex, utxo.OutIndex)
		txInput := modules.NewTxIn(prevOut, []byte{})
		payPTN.AddTxIn(txInput)
	}

	//ptn outputs
	if amountToken > 0 {
		payPTN.AddTxOut(modules.NewTxOut(amountToken, tokenengine.Instance.GenerateLockScript(toAddr), asset))
	}
	if change > 0 {
		payPTN.AddTxOut(modules.NewTxOut(change, tokenengine.Instance.GenerateLockScript(fromAddr), asset))
	}
	return payPTN, usedUtxo, nil
}

func buildRawTransferTx2(b Backend, tokenId, fromStr string, toAddrStr map[string]decimal.Decimal, gasFee decimal.Decimal, password string) (
	*modules.Transaction, []*modules.UtxoWithOutPoint, error) {
	//参数检查
	tokenAsset, err := modules.StringToAsset(tokenId)
	if err != nil {
		return nil, nil, err
	}

	if !gasFee.IsPositive() && b.EnableGasFee() {
		return nil, nil, fmt.Errorf("fee is ZERO ")
	}
	//
	fromAddr, err := parseAddressStr(fromStr, b.GetKeyStore(), password)
	if err != nil {
		fmt.Println(err.Error())
		return nil, nil, err
	}
	gasToken := dagconfig.DagConfig.GasToken
	gasAsset := dagconfig.DagConfig.GetGasToken()
	ptnAmount := uint64(0)
	toAddrAmt := make(map[common.Address]uint64)
	for toStr, toAmt := range toAddrStr {
		toAddr, err := parseAddressStr(toStr, b.GetKeyStore(), password)
		if err != nil {
			return nil, nil, err
		}
		amt := gasAsset.Uint64Amount(toAmt)
		toAddrAmt[toAddr] = amt
		if tokenId == gasToken {
			ptnAmount += amt
		}
	}
	tx := modules.NewTransaction([]*modules.Message{})

	usedUtxo1 := []*modules.UtxoWithOutPoint{}
	if b.EnableGasFee() {
		utxosPTN, err := b.GetPoolAddrUtxos(fromAddr, gasAsset.ToAsset())
		if err != nil {
			return nil, nil, fmt.Errorf("SelectUtxoFromDagAndPool utxo err:%s", err.Error())
		}
		feeAmount := gasAsset.Uint64Amount(gasFee)
		var pay1 *modules.PaymentPayload
		if ptnAmount == 0 {
			pay1, usedUtxo1, err = createPayment(fromAddr, fromAddr, ptnAmount, feeAmount, utxosPTN)
		} else {
			pay1, usedUtxo1, err = createPayment2(fromAddr, toAddrAmt, feeAmount, utxosPTN)
		}
		if err != nil {
			return nil, nil, err
		}
		tx.AddMessage(modules.NewMessage(modules.APP_PAYMENT, pay1))
		if tokenId == gasToken {
			return tx, usedUtxo1, nil
		}
	}
	log.Debugf("gas token[%s], transfer token[%s], start build payment1", gasToken, tokenId)
	//构造转移Token的Message1 m'k
	//utxosToken, err := SelectUtxoFromDagAndPool(dbUtxos, reqTxMapping, poolTxs, from, tokenId)
	utxosToken, err := b.GetPoolAddrUtxos(fromAddr, tokenAsset)
	if err != nil {
		return nil, nil, fmt.Errorf("SelectUtxoFromDagAndPool token utxo err:%s", err.Error())
	}
	pay2, usedUtxo2, err := createPayment2(fromAddr, toAddrAmt, 0, utxosToken)
	if err != nil {
		return nil, nil, err
	}
	tx.AddMessage(modules.NewMessage(modules.APP_PAYMENT, pay2))
	usedUtxo1 = append(usedUtxo1, usedUtxo2...)

	return tx, usedUtxo1, nil
}

func createPayment2(fromAddr common.Address, toAddr map[common.Address]uint64, feePTN uint64,
	utxosPTN map[modules.OutPoint]*modules.Utxo) (*modules.PaymentPayload, []*modules.UtxoWithOutPoint, error) {

	if len(utxosPTN) == 0 {
		log.Errorf("No PTN Utxo or No Token Utxo for %s", fromAddr.String())
		return nil, nil, fmt.Errorf("No Utxo found for %s", fromAddr.String())
	}

	//PTN
	utxoPTNView, asset := convertUtxoMap2Utxos(utxosPTN)
	amountToken := uint64(0)
	for _, amt := range toAddr {
		amountToken += amt
	}
	utxosPTNTaken, change, err := core.Select_utxo_Greedy(utxoPTNView, amountToken+feePTN)
	if err != nil {
		return nil, nil, fmt.Errorf("createPayment Select_utxo_Greedy utxo err:%s", err.Error())
	}
	usedUtxo := []*modules.UtxoWithOutPoint{}
	//ptn payment
	payPTN := &modules.PaymentPayload{}
	//ptn inputs
	for _, u := range utxosPTNTaken {
		utxo := u.(*modules.UtxoWithOutPoint)
		usedUtxo = append(usedUtxo, utxo)
		prevOut := &utxo.OutPoint // modules.NewOutPoint(txHash, utxo.MessageIndex, utxo.OutIndex)
		txInput := modules.NewTxIn(prevOut, []byte{})
		payPTN.AddTxIn(txInput)
	}

	//ptn outputs
	if amountToken > 0 {
		for addr, amt := range toAddr {
			payPTN.AddTxOut(modules.NewTxOut(amt, tokenengine.Instance.GenerateLockScript(addr), asset))
		}
	}
	if change > 0 {
		payPTN.AddTxOut(modules.NewTxOut(change, tokenengine.Instance.GenerateLockScript(fromAddr), asset))
	}
	return payPTN, usedUtxo, nil
}
func signRawTransaction(b Backend, rawTx *modules.Transaction, fromStr, password string, timeout *Int, hashType uint32,
	usedUtxo []*modules.UtxoWithOutPoint) error {
	if !b.EnableGasFee() {
		//no gas fee, enable nonce
		if rawTx.Nonce() == 0 {
			rawTx.SetNonce(uint64(time.Now().UnixNano()))
		}
		rawTx.SetVersion(1)
	}
	ks := b.GetKeyStore()
	//lockscript
	getPubKeyFn := func(addr common.Address) ([]byte, error) {
		return ks.GetPublicKey(addr)
	}
	//sign tx
	getSignFn := func(addr common.Address, msg []byte) ([]byte, error) {
		return ks.SignMessage(addr, msg)
	}
	utxoLockScripts := make(map[modules.OutPoint][]byte)
	for _, utxo := range usedUtxo {
		utxoLockScripts[utxo.OutPoint] = utxo.PkScript
	}
	fromAddr, err := parseAddressStr(fromStr, ks, password)
	if err != nil {
		return err
	}
	err = unlockKS(b, fromAddr, password, timeout)
	if err != nil {
		return err
	}
	//Sign tx
	_, err = tokenengine.Instance.SignTxAllPaymentInput(rawTx, hashType, utxoLockScripts, nil, getPubKeyFn, getSignFn)
	if err != nil {
		return err
	}
	//log for debug
	log.DebugDynamic(func() string {
		txJson, _ := json.Marshal(rawTx)
		return "SignedTx:" + string(txJson)
	})
	return nil
}

// submitTransaction is a helper function that submits tx to txPool and logs a message.
func submitTransaction(b Backend, tx *modules.Transaction) (common.Hash, error) {
	if tx.IsOnlyContractRequest() && tx.GetContractTxType() != modules.APP_CONTRACT_INVOKE_REQUEST {
		log.Debugf("[%s]submitTransaction, not invoke Tx", tx.RequestHash().String()[:8])
		reqId, err := b.SendContractInvokeReqTx(tx)
		return reqId, err
	}
	log.Debugf("[%s]submitTransaction, is invoke Tx", tx.RequestHash().String()[:8])
	//普通交易和系统合约交易，走交易池
	if err := b.SendTx(tx); err != nil {
		return common.Hash{}, err
	}
	return tx.Hash(), nil
}

type Int struct {
	i uint64
}

func (i *Int) Uint32() uint32 {
	if i == nil {
		return 0
	}
	return uint32(i.i)
}
func (i *Int) Uint64() uint64 {
	if i == nil {
		return 0
	}
	return i.i
}
func (d *Int) UnmarshalJSON(iBytes []byte) error {
	if string(iBytes) == "null" {
		return nil
	}
	if len(iBytes) == 0 {
		d.i = 0
		return nil
	}
	//log.Debugf("Int json[%s] hex:%x",string(iBytes),iBytes)
	iStr := string(iBytes)
	if iBytes[0] == byte('"') { // "1" -> 1
		iStr = string(iBytes[1 : len(iBytes)-1])
	}
	input, err := strconv.ParseUint(iStr, 10, 64)
	if err != nil {
		return err
	}
	d.i = input
	return nil
}
func signRawNoGasTx(b Backend, tx *modules.Transaction, addr common.Address, pwd string) (*modules.Transaction, error) {
	//no gas fee, enable nonce
	if tx.Nonce() == 0 {
		tx.SetNonce(uint64(time.Now().UnixNano()))
	}
	tx.SetVersion(1)
	keystore := b.GetKeyStore()
	if !keystore.IsUnlock(addr) {
		keystore.Unlock(accounts.Account{Address: addr}, pwd)
	}
	sign, err := keystore.SigData(tx, addr)
	if err != nil {
		return nil, err
	}
	pubKey, err := keystore.GetPublicKey(addr)
	if err != nil {
		return nil, err
	}
	ss := modules.SignatureSet{
		PubKey:    pubKey,
		Signature: sign,
	}
	signature := &modules.SignaturePayload{Signatures: []modules.SignatureSet{ss}}
	tx.AddMessage(modules.NewMessage(modules.APP_SIGNATURE, signature))
	return tx, nil
}
