/*
	This file is part of go-palletone.
	go-palletone is free software: you can redistribute it and/or modify
	it under the terms of the GNU General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.
	go-palletone is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.
	You should have received a copy of the GNU General Public License
	along with go-palletone.  If not, see <http://www.gnu.org/licenses/>.
*/
/*
 * Copyright IBM Corp. All Rights Reserved.
 * @author PalletOne core developers <dev@pallet.one>
 * @date 2018
 */

package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/palletone/go-palletone/common"
	"github.com/palletone/go-palletone/common/crypto"
	"github.com/palletone/go-palletone/common/log"
	"github.com/palletone/go-palletone/contracts/shim"
	pb "github.com/palletone/go-palletone/core/vmContractPub/protos/peer"
	"github.com/palletone/go-palletone/dag/errors"
	dm "github.com/palletone/go-palletone/dag/modules"

	"github.com/palletone/adaptor"
)

type ETHPort struct {
}

func (p *ETHPort) Init(stub shim.ChaincodeStubInterface) pb.Response {
	return shim.Success(nil)
}

func (p *ETHPort) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	f, args := stub.GetFunctionAndParameters()

	switch f {
	case "initDepositAddr":
		return p.InitDepositAddr(stub)

	case "setETHTokenAsset":
		if len(args) < 1 {
			return shim.Error("need 1 args (AssetStr)")
		}
		return p.SetETHTokenAsset(args[0], stub)
	case "getETHToken":
		fallthrough
	case "getETHTokenByAddr":
		if len(args) < 1 {
			return shim.Error("need 1 args (ETHAddr)")
		}
		return p.GetETHTokenByAddr(args[0], stub)
	case "getETHTokenByTxID":
		if len(args) < 1 {
			return shim.Error("need 1 args (ETHTransferTxID)")
		}
		return p.GetETHTokenByTxID(args[0], stub)

	case "setETHContract":
		if len(args) < 1 {
			return shim.Error("need 1 args (ETHContractAddr)")
		}
		return p.SetETHContract(args[0], stub)
	case "setOwner":
		if len(args) < 1 {
			return shim.Error("need 1 args (PTNAddr)")
		}
		return p.SetOwner(args[0], stub)

	case "withdrawPrepare":
		if len(args) < 1 {
			return shim.Error("need 1 args (ethAddr, [ethFee(>10000)])")
		}
		ethAddr := args[0]
		ethFee := uint64(0)
		if len(args) > 1 {
			ethFee, _ = strconv.ParseUint(args[1], 10, 64)
		}
		return p.WithdrawPrepare(ethAddr, ethFee, stub)
	case "withdrawETH":
		if len(args) < 1 {
			return shim.Error("need 1 args (reqid)")
		}
		return p.WithdrawETH(args[0], stub)
	case "withdrawFee":
		if len(args) < 1 {
			return shim.Error("need 1 args (ethAddr)")
		}
		return p.WithdrawFee(args[0], stub)

	case "get":
		if len(args) < 1 {
			return shim.Error("need 1 args (Key)")
		}
		return p.Get(stub, args[0])
	default:
		jsonResp := "{\"Error\":\"Unknown function " + f + "\"}"
		return shim.Error(jsonResp)
	}
}

type JuryMsgAddr struct {
	Address string
	Answer  []byte
}

//todo modify conforms 15
const Confirms = uint(15)

const symbolsJuryAddress = "juryPubkeyAddress"

const symbolsETHAsset = "eth_asset"
const symbolsETHContract = "eth_contract"

const symbolsDeposit = "deposit_"

const symbolsWithdrawPrepare = "withdrawPrepare_"

const symbolsWithdrawFee = "withdrawfee_"
const symbolsOwner = "owner_"

const symbolsWithdraw = "withdraw_"

const consultM = 3
const consultN = 4

const jsonResp1 = "{\"Error\":\"Failed to get contractAddr, need set contractAddr\"}"

// contractABI is same, but contractAddr is not
const contractABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"reqid\",\"type\":\"string\"}],\"name\":\"getmultisig\",\"outputs\":[{\"name\":\"\",\"type\":\"uint8\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"suicideto\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"ptnaddr\",\"type\":\"string\"}],\"name\":\"deposit\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"my_eth_bal\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"addra\",\"type\":\"address\"},{\"name\":\"addrb\",\"type\":\"address\"},{\"name\":\"addrc\",\"type\":\"address\"},{\"name\":\"addrd\",\"type\":\"address\"}],\"name\":\"setaddrs\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"recver\",\"type\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\"},{\"name\":\"reqid\",\"type\":\"string\"},{\"name\":\"sigstr1\",\"type\":\"bytes\"},{\"name\":\"sigstr2\",\"type\":\"bytes\"},{\"name\":\"sigstr3\",\"type\":\"bytes\"}],\"name\":\"withdraw\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"addra\",\"type\":\"address\"},{\"name\":\"addrb\",\"type\":\"address\"},{\"name\":\"addrc\",\"type\":\"address\"},{\"name\":\"addrd\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"fallback\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"token\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"user\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"ptnaddr\",\"type\":\"string\"}],\"name\":\"Deposit\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"token\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"user\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"recver\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"reqid\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"confirmvalue\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"state\",\"type\":\"string\"}],\"name\":\"Withdraw\",\"type\":\"event\"}]"

func consult(stub shim.ChaincodeStubInterface, content []byte, myAnswer []byte) ([]byte, error) {
	sendResult, err := stub.SendJury(2, content, myAnswer)
	if err != nil {
		log.Debugf("SendJury err: %s", err.Error())
		return nil, errors.New("SendJury failed")
	}
	log.Debugf("sendResult: %s", common.Bytes2Hex(sendResult))
	recvResult, err := stub.RecvJury(2, content, 2)
	if err != nil {
		recvResult, err = stub.RecvJury(2, content, 2)
		if err != nil {
			log.Debugf("RecvJury err: %s", err.Error())
			return nil, errors.New("RecvJury failed")
		}
	}
	log.Debugf("recvResult: %s", string(recvResult))
	return recvResult, nil
}

type pubkeyAddr struct {
	addr   string
	pubkey []byte
}
type pubkeyAddrWrapper struct {
	pubAddr []pubkeyAddr
	by      func(p, q *pubkeyAddr) bool
}
type SortBy func(p, q *pubkeyAddr) bool

func (pw pubkeyAddrWrapper) Len() int { // 重写 Len() 方法
	return len(pw.pubAddr)
}
func (pw pubkeyAddrWrapper) Swap(i, j int) { // 重写 Swap() 方法
	pw.pubAddr[i], pw.pubAddr[j] = pw.pubAddr[j], pw.pubAddr[i]
}
func (pw pubkeyAddrWrapper) Less(i, j int) bool { // 重写 Less() 方法
	return pw.by(&pw.pubAddr[i], &pw.pubAddr[j])
}

func sortPubAddr(thePubAddr []pubkeyAddr, by SortBy) { // sortPubAddr 方法
	sort.Sort(pubkeyAddrWrapper{thePubAddr, by})
}

func addrIncrease(p, q *pubkeyAddr) bool {
	return p.addr < q.addr // addr increase sort
}

func (p *ETHPort) InitDepositAddr(stub shim.ChaincodeStubInterface) pb.Response {
	//
	saveResult, _ := stub.GetState(symbolsJuryAddress)
	if len(saveResult) != 0 {
		return shim.Error("DepositAddr has been init")
	}

	//Method:GetJuryETHAddr, return address string
	juryAddr, err := stub.OutChainCall("eth", "GetJuryAddr", []byte(""))
	if err != nil {
		log.Debugf("OutChainCall GetJuryETHAddr err: %s", err.Error())
		return shim.Error("OutChainCall GetJuryETHAddr failed" + err.Error())
	}

	result, err := stub.OutChainCall("eth", "GetJuryPubkey", []byte(""))
	if err != nil {
		log.Debugf("OutChainCall GetJuryPubkey err: %s", err.Error())
		return shim.Error("OutChainCall GetJuryPubkey failed" + err.Error())
	}
	var juryPubkey adaptor.GetPublicKeyOutput
	err = json.Unmarshal(result, &juryPubkey)
	if err != nil {
		log.Debugf("OutChainCall GetJuryPubkey Unmarshal err: %s", err.Error())
		return shim.Error("OutChainCall GetJuryPubkey Unmarshal failed" + err.Error())
	}

	myPubkeyAddr := pubkeyAddr{string(juryAddr), juryPubkey.PublicKey}
	myPubkeyAddrByte, _ := json.Marshal(myPubkeyAddr)
	//
	recvResult, err := consult(stub, []byte("juryETHPubkey"), myPubkeyAddrByte)
	if err != nil {
		log.Debugf("consult juryETHPubkey failed: " + err.Error())
		return shim.Error("consult juryETHPubkey failed: " + err.Error())
	}
	var juryMsg []JuryMsgAddr
	err = json.Unmarshal(recvResult, &juryMsg)
	if err != nil {
		return shim.Error("Unmarshal result failed: " + err.Error())
	}
	if len(juryMsg) != consultN {
		return shim.Error("RecvJury result's len not enough")
	}

	//
	pubkeyAddrs := make([]pubkeyAddr, 0, len(juryMsg))
	for i := range juryMsg {
		var onePubkeyAddr pubkeyAddr
		err := json.Unmarshal(juryMsg[i].Answer, &onePubkeyAddr)
		if err != nil {
			continue
		}
		pubkeyAddrs = append(pubkeyAddrs, onePubkeyAddr)
	}
	if len(pubkeyAddrs) != consultN {
		return shim.Error("pubkeyAddrs result's len not enough")
	}
	sortPubAddr(pubkeyAddrs, addrIncrease)

	address := make([]string, 0, len(pubkeyAddrs))
	//pubkeys := make([][]byte, 0, len(pubkeyAddrs))
	for i := range pubkeyAddrs {
		address = append(address, pubkeyAddrs[i].addr)
		//pubkeys = append(pubkeys, pubkeyAddrs[i].pubkey)
	}
	addressJson, err := json.Marshal(address)
	if err != nil {
		return shim.Error("address Marshal failed: " + err.Error())
	}

	pubkeyAddrsJson, err := json.Marshal(pubkeyAddrs)
	if err != nil {
		return shim.Error("pubkeyAddrs Marshal failed: " + err.Error())
	}
	// Write the state to the ledger
	err = stub.PutState(symbolsJuryAddress, pubkeyAddrsJson)
	if err != nil {
		return shim.Error("write " + symbolsJuryAddress + " failed: " + err.Error())
	}
	return shim.Success(addressJson)
}

func getETHAddrs(stub shim.ChaincodeStubInterface) []pubkeyAddr {
	result, _ := stub.GetState(symbolsJuryAddress)
	if len(result) == 0 {
		return []pubkeyAddr{}
	}
	var pubkeyAddrs []pubkeyAddr
	err := json.Unmarshal(result, &pubkeyAddrs)
	if err != nil {
		return []pubkeyAddr{}
	}
	return pubkeyAddrs
}

func (p *ETHPort) SetETHTokenAsset(assetStr string, stub shim.ChaincodeStubInterface) pb.Response {
	err := stub.PutState(symbolsETHAsset, []byte(assetStr))
	if err != nil {
		return shim.Error("write symbolsETHAsset failed: " + err.Error())
	}
	return shim.Success([]byte("Success"))
}

func getETHTokenAsset(stub shim.ChaincodeStubInterface) *dm.Asset {
	result, _ := stub.GetState(symbolsETHAsset)
	if len(result) == 0 {
		return nil
	}
	asset, _ := dm.StringToAsset(string(result))
	log.Debugf("resultHex %s, asset: %s", common.Bytes2Hex(result), asset.String())

	return asset
}

func (p *ETHPort) SetETHContract(ethContractAddr string, stub shim.ChaincodeStubInterface) pb.Response {
	//
	saveResult, _ := stub.GetState(symbolsETHContract)
	if len(saveResult) != 0 {
		return shim.Error("TokenAsset has been init")
	}

	err := stub.PutState(symbolsETHContract, []byte(ethContractAddr))
	if err != nil {
		return shim.Error("write symbolsETHContract failed: " + err.Error())
	}
	return shim.Success([]byte("Success"))
}

func (p *ETHPort) SetOwner(ptnAddr string, stub shim.ChaincodeStubInterface) pb.Response {
	err := stub.PutState(symbolsOwner, []byte(ptnAddr))
	if err != nil {
		return shim.Error("write symbolsOwner failed: " + err.Error())
	}
	return shim.Success([]byte("Success"))
}
func getETHContract(stub shim.ChaincodeStubInterface) string {
	result, _ := stub.GetState(symbolsETHContract)
	if len(result) == 0 {
		return ""
	}
	log.Debugf("contractAddr: %s", string(result))

	return string(result)
}

func GetAddrHistory(ethAddrFrom, mapAddrTo string, stub shim.ChaincodeStubInterface) (*adaptor.GetAddrTxHistoryOutput, error) {
	input := adaptor.GetAddrTxHistoryInput{FromAddress: ethAddrFrom, ToAddress: mapAddrTo, Asset: "ETH",
		AddressLogicAndOr: true}
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	//
	result, err := stub.OutChainCall("eth", "GetAddrTxHistory", inputBytes)
	if err != nil {
		return nil, errors.New("GetAddrHistory error: " + err.Error())
	}
	log.Debugf("result : %s", string(result))
	//
	var output adaptor.GetAddrTxHistoryOutput
	err = json.Unmarshal(result, &output)
	if err != nil {
		return nil, err
	}
	return &output, nil
}

func getHeight(stub shim.ChaincodeStubInterface) (uint, error) {
	//
	input := adaptor.GetBlockInfoInput{Latest: true} //get best hight
	//
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return 0, err
	}
	//adaptor.
	result, err := stub.OutChainCall("eth", "GetBlockInfo", inputBytes)
	if err != nil {
		return 0, errors.New("GetBlockInfo error: " + err.Error())
	}
	//
	var output adaptor.GetBlockInfoOutput
	err = json.Unmarshal(result, &output)
	if err != nil {
		return 0, err
	}

	if output.Block.BlockHeight == 0 {
		return 0, errors.New("{\"Error\":\"Failed to get eth height\"}")
	}

	return output.Block.BlockHeight, nil
}

func (p *ETHPort) GetETHTokenByAddr(ethAddr string, stub shim.ChaincodeStubInterface) pb.Response {
	//
	mapAddr := getETHContract(stub)
	if mapAddr == "" {
		return shim.Error(jsonResp1)
	}

	//get the mapping ptnAddr
	ptnAddr, err := getPTNMapAddr(mapAddr, ethAddr, stub)
	if err != nil {
		log.Debugf("getPTNMapAddr failed: %s", err.Error())
		return shim.Error(err.Error())
	}

	txResults, err := GetAddrHistory(ethAddr, mapAddr, stub)
	if err != nil {
		log.Debugf("GetAddrHistory failed: %s", err.Error())
		return shim.Error(err.Error())
	}

	curHeight, err := getHeight(stub)
	if curHeight == 0 || err != nil {
		return shim.Error("getHeight failed")
	}

	var amt uint64
	for _, txResult := range txResults.Txs {
		txIDHex := hex.EncodeToString(txResult.TxID)
		//check confirms
		if curHeight-txResult.BlockHeight < Confirms {
			log.Debugf("Need more confirms %s", txIDHex)
			continue
		}
		//
		result, _ := stub.GetState(symbolsDeposit + txIDHex)
		if len(result) != 0 {
			log.Debugf("The tx %s has been payout", txIDHex)
			continue
		}
		log.Debugf("The tx %s need be payout", txIDHex)

		//check token amount
		bigIntAmout := txResult.Amount.Amount.Div(txResult.Amount.Amount, big.NewInt(1e10)) //eth's decimal is 18, ethToken in PTN is decimal is 8
		amt += txResult.Amount.Amount.Uint64()

		//save payout history
		err = stub.PutState(symbolsDeposit+txIDHex, []byte(ptnAddr+"-"+bigIntAmout.String()))
		if err != nil {
			log.Debugf("write symbolsPayout failed: %s", err.Error())
			return shim.Error("write symbolsPayout failed: " + err.Error())
		}

	}

	if amt == 0 {
		log.Debugf("You need deposit or need wait confirm")
		return shim.Error("You need deposit or need wait confirm")
	}

	//
	ethTokenAsset := getETHTokenAsset(stub)
	if ethTokenAsset == nil {
		return shim.Error("need call setETHTokenAsset()")
	}
	invokeTokens := new(dm.AmountAsset)
	invokeTokens.Amount = amt
	invokeTokens.Asset = ethTokenAsset
	err = stub.PayOutToken(ptnAddr, invokeTokens, 0)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to call stub.PayOutToken\"}"
		return shim.Error(jsonResp)
	}

	return shim.Success([]byte("get success"))
}

func getPTNMapAddr(mapAddr, fromAddr string, stub shim.ChaincodeStubInterface) (string, error) {
	var input adaptor.GetPalletOneMappingAddressInput
	input.MappingDataSource = mapAddr
	input.ChainAddress = fromAddr

	inputBytes, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	//
	result, err := stub.OutChainCall("eth", "GetPalletOneMappingAddress", inputBytes)
	if err != nil {
		return "", errors.New("GetPalletOneMappingAddress failed: " + err.Error())
	}
	//
	var output adaptor.GetPalletOneMappingAddressOutput
	err = json.Unmarshal(result, &output)
	if err != nil {
		return "", err
	}
	if output.PalletOneAddress == "" {
		return "", errors.New("GetPalletOneMappingAddress result empty")
	}

	return output.PalletOneAddress, nil
}

func GetETHTx(txID []byte, stub shim.ChaincodeStubInterface) (*adaptor.GetTransferTxOutput, error) {
	input := adaptor.GetTransferTxInput{TxID: txID}
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	//
	result, err := stub.OutChainCall("eth", "GetTransferTx", inputBytes)
	if err != nil {
		return nil, errors.New("GetTransferTx error: " + err.Error())
	}
	log.Debugf("result : %s", string(result))

	//
	var output adaptor.GetTransferTxOutput
	err = json.Unmarshal(result, &output)
	if err != nil {
		return nil, err
	}
	return &output, nil
}

func (p *ETHPort) GetETHTokenByTxID(ethTxID string, stub shim.ChaincodeStubInterface) pb.Response {
	//
	if "0x" == ethTxID[0:2] || "0X" == ethTxID[0:2] {
		ethTxID = ethTxID[2:]
	}
	result, _ := stub.GetState(symbolsDeposit + ethTxID)
	if len(result) != 0 {
		log.Debugf("The tx has been payout")
		return shim.Error("The tx has been payout")
	}

	//get sender receiver amount
	txIDByte, err := hex.DecodeString(ethTxID)
	if err != nil {
		log.Debugf("txid invalid: %s", err.Error())
		return shim.Error(fmt.Sprintf("txid invalid: %s", err.Error()))
	}

	mapAddr := getETHContract(stub)
	if mapAddr == "" {
		return shim.Error(jsonResp1)
	}
	txResult, err := GetETHTx(txIDByte, stub)
	if err != nil {
		log.Debugf("GetETHTx failed: %s", err.Error())
		return shim.Error(err.Error())
	}
	//check tx status
	if !txResult.Tx.IsSuccess {
		log.Debugf("The tx is failed")
		return shim.Error("The tx is failed")
	}
	//check contract address, must be ptn eth port contract address
	if strings.ToLower(txResult.Tx.TargetAddress) != mapAddr {
		log.Debugf("The tx is't transfer to eth port contract")
		return shim.Error("The tx is't transfer to eth port contract")
	}

	//get the mapping ptnAddr
	ptnAddr, err := getPTNMapAddr(mapAddr, txResult.Tx.FromAddress, stub)
	if err != nil {
		log.Debugf("getPTNMapAddr failed: %s", err.Error())
		return shim.Error(err.Error())
	}

	bigIntAmount := txResult.Tx.Amount.Amount
	bigIntAmount = bigIntAmount.Div(bigIntAmount, big.NewInt(1e10)) //ethToken in PTN is decimal is 8
	//
	err = stub.PutState(symbolsDeposit+ethTxID, []byte(ptnAddr+"-"+bigIntAmount.String()))
	if err != nil {
		log.Debugf("PutState sigHash failed err: %s", err.Error())
		return shim.Error("PutState sigHash failed")
	}

	ethAmount := bigIntAmount.Uint64()
	if ethAmount == 0 {
		return shim.Error("You need deposit or need wait confirm")
	}
	//
	ethTokenAsset := getETHTokenAsset(stub)
	if ethTokenAsset == nil {
		return shim.Error("need call setETHTokenAsset()")
	}
	invokeTokens := new(dm.AmountAsset)
	invokeTokens.Amount = ethAmount
	invokeTokens.Asset = ethTokenAsset
	err = stub.PayOutToken(ptnAddr, invokeTokens, 0)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to call stub.PayOutToken\"}"
		return shim.Error(jsonResp)
	}

	return shim.Success([]byte("get success"))
}

type WithdrawPrepare struct {
	EthAddr   string
	EthAmount uint64
	EthFee    uint64
}

func (p *ETHPort) WithdrawPrepare(ethAddr string, ethFee uint64, stub shim.ChaincodeStubInterface) pb.Response {
	if ethFee <= 10000 { //0.0001eth
		ethFee = 10000
	}
	//
	ethTokenAsset := getETHTokenAsset(stub)
	if ethTokenAsset == nil {
		return shim.Error("need call setETHTokenAsset()")
	}
	//contractAddr
	_, contractAddr := stub.GetContractID()

	//check token
	invokeTokens, err := stub.GetInvokeTokens()
	if err != nil {
		jsonResp := "{\"Error\":\"GetInvokeTokens failed\"}"
		return shim.Error(jsonResp)
	}

	ethTokenAmount := uint64(0)
	log.Debugf("contractAddr %s", contractAddr)
	for i := 0; i < len(invokeTokens); i++ {
		log.Debugf("invokeTokens[i].Address %s", invokeTokens[i].Address)
		if invokeTokens[i].Address == contractAddr {
			if invokeTokens[i].Asset.AssetId == ethTokenAsset.AssetId {
				ethTokenAmount += invokeTokens[i].Amount
			}
		}
	}
	if ethTokenAmount == 0 {
		log.Debugf("You need send contractAddr ethToken")
		jsonResp := "{\"Error\":\"You need send contractAddr ethToken\"}"
		return shim.Error(jsonResp)
	}
	log.Debugf("ethTokenAmount %d", ethTokenAmount)

	reqid := stub.GetTxID()
	// 产生交易
	rawTx := fmt.Sprintf("%s %d %s", ethAddr, ethTokenAmount, reqid)
	log.Debugf("rawTx:%s", rawTx)

	tempHash := crypto.Keccak256([]byte(rawTx), []byte("prepare"))
	tempHashHex := fmt.Sprintf("%x", tempHash)
	log.Debugf("tempHashHex:%s", tempHashHex)

	//协商交易
	recvResult, err := consult(stub, []byte(tempHashHex), []byte("rawTx"))
	if err != nil {
		log.Debugf("consult rawTx failed: " + err.Error())
		return shim.Error("consult rawTx failed: " + err.Error())
	}
	var juryMsg []JuryMsgAddr
	err = json.Unmarshal(recvResult, &juryMsg)
	if err != nil {
		log.Debugf("Unmarshal rawTx result failed: " + err.Error())
		return shim.Error("Unmarshal rawTx result failed: " + err.Error())
	}
	if len(juryMsg) < consultM {
		log.Debugf("RecvJury rawTx result's len not enough")
		return shim.Error("RecvJury rawTx result's len not enough")
	}

	// 记录Prepare
	var prepare WithdrawPrepare
	prepare.EthAddr = ethAddr
	prepare.EthAmount = ethTokenAmount
	prepare.EthFee = ethFee
	prepareByte, err := json.Marshal(prepare)
	if err != nil {
		log.Debugf("Marshal prepare failed: " + err.Error())
		return shim.Error("Marshal prepare failed: " + err.Error())
	}
	err = stub.PutState(symbolsWithdrawPrepare+reqid, prepareByte)
	if err != nil {
		log.Debugf("save symbolsWithdrawPrepare failed: " + err.Error())
		return shim.Error("save symbolsWithdrawPrepare failed: " + err.Error())
	}

	updateFee(ethFee, stub)

	return shim.Success([]byte("Withdraw is ready, please invoke withdrawETH"))
}

func updateFee(fee uint64, stub shim.ChaincodeStubInterface) {
	feeCur := uint64(0)
	result, _ := stub.GetState(symbolsWithdrawFee)
	if len(result) != 0 {
		log.Debugf("updateFee fee current : %s ", string(result))
		feeCur, _ = strconv.ParseUint(string(result), 10, 64)
	}
	fee += feeCur
	feeStr := fmt.Sprintf("%d", fee)
	err := stub.PutState(symbolsWithdrawFee, []byte(feeStr))
	if err != nil {
		log.Debugf("updateFee failed: " + err.Error())
	}
}

func getFee(stub shim.ChaincodeStubInterface) uint64 {
	feeCur := uint64(0)
	result, _ := stub.GetState(symbolsWithdrawFee)
	if len(result) != 0 {
		log.Debugf("getFee fee current : %s ", string(result))
		feeCur, _ = strconv.ParseUint(string(result), 10, 64)
	}
	return feeCur
}

func LeftPadBytes(slice []byte, l int) []byte {
	if l <= len(slice) {
		return slice
	}

	padded := make([]byte, l)
	copy(padded[l-len(slice):], slice)

	return padded
}

// Lengths of hashes and addresses in bytes.
const (
	// HashLength is the expected length of the hash
	HashLength = 32
	// AddressLength is the expected length of the address
	AddressLength = 20
)

// Address represents the 20 byte address of an Ethereum account.
type ETHAddress [AddressLength]byte

// BytesToAddress returns Address with value b.
// If b is larger than len(h), b will be cropped from the left.
func BytesToAddress(b []byte) ETHAddress {
	var a ETHAddress
	a.SetBytes(b)
	return a
}

// SetBytes sets the address to the value of b.
// If b is larger than len(a) it will panic.
func (a *ETHAddress) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-AddressLength:]
	}
	copy(a[AddressLength-len(b):], b)
}

// HexToAddress returns Address with byte values of s.
// If s is larger than len(h), s will be cropped from the left.
func HexToAddress(s string) ETHAddress { return BytesToAddress(FromHex(s)) }

// FromHex returns the bytes represented by the hexadecimal string s.
// s may be prefixed with "0x".
func FromHex(s string) []byte {
	if len(s) > 1 {
		if s[0:2] == "0x" || s[0:2] == "0X" {
			s = s[2:]
		}
	}
	if len(s)%2 == 1 {
		s = "0" + s
	}
	return Hex2Bytes(s)
}

// Hex2Bytes returns the bytes represented by the hexadecimal string str.
func Hex2Bytes(str string) []byte {
	h, _ := hex.DecodeString(str)
	return h
}

// Bytes gets the string representation of the underlying address.
func (a ETHAddress) Bytes() []byte { return a[:] }

func getPadderBytes(contractAddr, reqid, recvAddr string, ethAmount uint64) []byte {
	var allBytes []byte
	ethContractAddr := HexToAddress(contractAddr)
	allBytes = append(allBytes, ethContractAddr.Bytes()...)

	ethRecvAddr := HexToAddress(recvAddr)
	allBytes = append(allBytes, ethRecvAddr.Bytes()...)

	paramBigInt := new(big.Int)
	paramBigInt.SetUint64(ethAmount)
	paramBigIntBytes := LeftPadBytes(paramBigInt.Bytes(), 32)
	allBytes = append(allBytes, paramBigIntBytes...)

	allBytes = append(allBytes, []byte(reqid)...)
	return allBytes
}
func calSig(msg []byte, stub shim.ChaincodeStubInterface) ([]byte, error) {
	//

	input := adaptor.SignMessageInput{Message: msg}
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return []byte{}, err
	}

	//
	result, err := stub.OutChainCall("eth", "SignMessage", inputBytes)
	if err != nil {
		return []byte{}, errors.New("SignMessage error" + err.Error())
	}
	//
	var sigResult adaptor.SignMessageOutput
	err = json.Unmarshal(result, &sigResult)
	if err != nil {
		return []byte{}, err
	}
	return sigResult.Signature, nil
}

func recoverAddr(msg, pubkey, sig []byte, stub shim.ChaincodeStubInterface) (bool, error) {
	ethTX := adaptor.VerifySignatureInput{Message: msg, Signature: sig, PublicKey: pubkey}
	reqBytes, err := json.Marshal(ethTX)
	if err != nil {
		return false, err
	}
	//
	result, err := stub.OutChainCall("eth", "VerifySignature", reqBytes)
	if err != nil {
		return false, errors.New("RecoverAddr error" + err.Error())
	}
	//
	var recoverResult adaptor.VerifySignatureOutput
	err = json.Unmarshal(result, &recoverResult)
	if err != nil {
		return false, err
	}
	return recoverResult.Pass, nil
}

func verifySigs(msg []byte, juryMsg []JuryMsgAddr, pubkeyAddrs []pubkeyAddr, stub shim.ChaincodeStubInterface) []string {
	//
	var sigs []string
	for i := range juryMsg {
		var onePubkeySig pubkeySig
		err := json.Unmarshal(juryMsg[i].Answer, &onePubkeySig)
		if err != nil {
			continue
		}
		isJuryETHPubkey := false
		for j := range pubkeyAddrs {
			if bytes.Equal(pubkeyAddrs[j].pubkey, onePubkeySig.pubkey) {
				isJuryETHPubkey = true
			}
		}
		if !isJuryETHPubkey {
			continue
		}
		valid, err := recoverAddr(msg, onePubkeySig.pubkey, onePubkeySig.sig, stub)
		if err != nil {
			continue
		}
		if valid {
			sigs = append(sigs, string(juryMsg[i].Answer))
		}
	}
	//sort
	a := sort.StringSlice(sigs[0:])
	sort.Sort(a)
	return sigs
}

type Withdraw struct {
	EthAddr   string
	EthAmount uint64
	EthFee    uint64
	Sigs      []string
}

type pubkeySig struct {
	pubkey []byte
	sig    []byte
}

func (p *ETHPort) WithdrawETH(reqid string, stub shim.ChaincodeStubInterface) pb.Response {
	if "0x" != reqid[0:2] {
		reqid = "0x" + reqid
	}

	result, _ := stub.GetState(symbolsWithdrawPrepare + reqid)
	if len(result) == 0 {
		return shim.Error("Please invoke withdrawPrepare first")
	}

	// 检查交易
	var prepare WithdrawPrepare
	err := json.Unmarshal(result, &prepare)
	if nil != err {
		jsonResp := "Unmarshal WithdrawPrepare failed"
		return shim.Error(jsonResp)
	}

	contractAddr := getETHContract(stub)
	if contractAddr == "" {
		return shim.Error(jsonResp1)
	}

	//
	padderBytes := getPadderBytes(contractAddr, reqid, prepare.EthAddr, prepare.EthAmount-prepare.EthFee)

	// 计算签名
	sig, err := calSig(padderBytes, stub)
	if err != nil {
		return shim.Error("calSig failed: " + err.Error())
	}
	log.Debugf("sig: %s", sig)

	resultPubkey, err := stub.OutChainCall("eth", "GetJuryPubkey", []byte(""))
	if err != nil {
		log.Debugf("OutChainCall GetJuryPubkey err: %s", err.Error())
		return shim.Error("OutChainCall GetJuryPubkey failed" + err.Error())
	}
	var juryPubkey adaptor.GetPublicKeyOutput
	err = json.Unmarshal(resultPubkey, &juryPubkey)
	if err != nil {
		log.Debugf("OutChainCall GetJuryPubkey Unmarshal err: %s", err.Error())
		return shim.Error("OutChainCall GetJuryPubkey Unmarshal failed" + err.Error())
	}

	//
	reqidNew := stub.GetTxID()
	rawTx := fmt.Sprintf("%s %d %s", prepare.EthAddr, prepare.EthAmount-prepare.EthFee, reqidNew)
	tempHash := crypto.Keccak256([]byte(rawTx))
	tempHashHex := fmt.Sprintf("%x", tempHash)
	log.Debugf("tempHashHex:%s", tempHashHex)

	myPubkeySig := pubkeySig{pubkey: juryPubkey.PublicKey, sig: sig}
	myPubkeySigBytes, _ := json.Marshal(myPubkeySig)

	//用交易哈希协商交易签名，作适当安全防护
	recvResult, err := consult(stub, []byte(tempHashHex), myPubkeySigBytes)
	if err != nil {
		log.Debugf("consult sig failed: " + err.Error())
		return shim.Error("consult sig failed: " + err.Error())
	}
	var juryMsg []JuryMsgAddr
	err = json.Unmarshal(recvResult, &juryMsg)
	if err != nil {
		log.Debugf("Unmarshal sig result failed: " + err.Error())
		return shim.Error("Unmarshal sig result failed: " + err.Error())
	}
	//stub.PutState("recvResult", recvResult)
	if len(juryMsg) < consultM {
		log.Debugf("RecvJury sig result's len not enough")
		return shim.Error("RecvJury sig result's len not enough")
	}

	pubkeyAddrs := getETHAddrs(stub)
	if len(pubkeyAddrs) != consultN {
		log.Debugf("getETHAddrs result's len not enough")
		return shim.Error("getETHAddrs result's len not enough")
	}

	sigs := verifySigs(padderBytes, juryMsg, pubkeyAddrs, stub)
	if len(sigs) < consultM {
		log.Debugf("verifySigs result's len not enough")
		return shim.Error("verifySigs result's len not enough")
	}
	sigsStr := sigs[0]
	for i := 1; i < consultM; i++ {
		sigsStr = sigsStr + sigs[i]
	}
	sigHash := crypto.Keccak256([]byte(sigsStr))
	sigHashHex := fmt.Sprintf("%x", sigHash)
	log.Debugf("start consult sigHashHex %s", sigHashHex)

	//用签名列表的哈希协商交易签名，作适当安全防护
	txResult, err := consult(stub, []byte(sigHashHex), []byte("sigHash"))
	if err != nil {
		log.Debugf("consult sigHash failed: " + err.Error())
		return shim.Error("consult sigHash failed: " + err.Error())
	}
	var txJuryMsg []JuryMsgAddr
	err = json.Unmarshal(txResult, &txJuryMsg)
	if err != nil {
		log.Debugf("Unmarshal sigHash result failed: " + err.Error())
		return shim.Error("Unmarshal sigHash result failed: " + err.Error())
	}
	if len(txJuryMsg) < consultM {
		log.Debugf("RecvJury sigHash result's len not enough")
		return shim.Error("RecvJury sigHash result's len not enough")
	}
	//协商两次 保证协商一致后才写入签名结果
	txResult2, err := consult(stub, []byte(sigHashHex+"twice"), []byte("sigHash2"))
	if err != nil {
		log.Debugf("consult sigHash2 failed: " + err.Error())
		return shim.Error("consult sigHash2 failed: " + err.Error())
	}
	var txJuryMsg2 []JuryMsgAddr
	err = json.Unmarshal(txResult2, &txJuryMsg2)
	if err != nil {
		log.Debugf("Unmarshal sigHash2 result failed: " + err.Error())
		return shim.Error("Unmarshal sigHash2 result failed: " + err.Error())
	}
	if len(txJuryMsg2) < consultM {
		log.Debugf("RecvJury sigHash2 result's len not enough")
		return shim.Error("RecvJury sigHash2 result's len not enough")
	}

	//记录签名
	var withdraw Withdraw
	withdraw.EthAddr = prepare.EthAddr
	withdraw.EthAmount = prepare.EthAmount
	withdraw.EthFee = prepare.EthFee
	withdraw.Sigs = append(withdraw.Sigs, sigs[0:consultM]...)
	withdrawBytes, err := json.Marshal(withdraw)
	if err != nil {
		return shim.Error(err.Error())
	}
	err = stub.PutState(symbolsWithdraw+reqidNew, withdrawBytes)
	if err != nil {
		log.Debugf("save withdraw failed: " + err.Error())
		return shim.Error("save withdraw failed: " + err.Error())
	}

	//删除Prepare
	err = stub.DelState(symbolsWithdrawPrepare + reqid)
	if err != nil {
		log.Debugf("delete WithdrawPrepare failed: " + err.Error())
		return shim.Error("delete WithdrawPrepare failed: " + err.Error())
	}

	return shim.Success(withdrawBytes)
}

func (p *ETHPort) WithdrawFee(ethAddr string, stub shim.ChaincodeStubInterface) pb.Response {
	//
	invokeAddr, err := stub.GetInvokeAddress()
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get invoke address\"}"
		return shim.Error(jsonResp)
	}
	result, _ := stub.GetState(symbolsOwner)
	if len(result) == 0 {
		return shim.Error("Owner is not set")
	}
	if string(result) != invokeAddr.String() {
		return shim.Error("Must is the Owner")
	}

	//
	ethAmount := getFee(stub)
	if ethAmount == 0 {
		jsonResp := "{\"Error\":\"fee is 0\"}"
		return shim.Error(jsonResp)
	}
	contractAddr := getETHContract(stub)
	if contractAddr == "" {
		return shim.Error(jsonResp1)
	}

	//
	reqid := stub.GetTxID()

	padderBytes := getPadderBytes(contractAddr, reqid, ethAddr, ethAmount)

	// 计算签名
	sig, err := calSig(padderBytes, stub)
	if err != nil {
		return shim.Error("calSig failed: " + err.Error())
	}
	log.Debugf("sig:%s", sig)

	resultPubkey, err := stub.OutChainCall("eth", "GetJuryPubkey", []byte(""))
	if err != nil {
		log.Debugf("OutChainCall GetJuryPubkey err: %s", err.Error())
		return shim.Error("OutChainCall GetJuryPubkey failed" + err.Error())
	}
	var juryPubkey adaptor.GetPublicKeyOutput
	err = json.Unmarshal(resultPubkey, &juryPubkey)
	if err != nil {
		log.Debugf("OutChainCall GetJuryPubkey Unmarshal err: %s", err.Error())
		return shim.Error("OutChainCall GetJuryPubkey Unmarshal failed" + err.Error())
	}

	rawTx := fmt.Sprintf("%s %d %s", ethAddr, ethAmount, reqid)
	tempHash := crypto.Keccak256([]byte(rawTx))
	tempHashHex := fmt.Sprintf("%x", tempHash)
	log.Debugf("tempHashHex:%s", tempHashHex)

	myPubkeySig := pubkeySig{pubkey: juryPubkey.PublicKey, sig: sig}
	myPubkeySigBytes, _ := json.Marshal(myPubkeySig)

	//用交易哈希协商交易签名，作适当安全防护
	recvResult, err := consult(stub, []byte(tempHashHex), myPubkeySigBytes)
	if err != nil {
		log.Debugf("consult sig failed: " + err.Error())
		return shim.Error("consult sig failed: " + err.Error())
	}
	var juryMsg []JuryMsgAddr
	err = json.Unmarshal(recvResult, &juryMsg)
	if err != nil {
		log.Debugf("Unmarshal sig result failed: " + err.Error())
		return shim.Error("Unmarshal sig result failed: " + err.Error())
	}
	//stub.PutState("recvResult", recvResult)
	if len(juryMsg) < consultM {
		log.Debugf("RecvJury sig result's len not enough")
		return shim.Error("RecvJury sig result's len not enough")
	}

	pubkeyAddrs := getETHAddrs(stub)
	if len(pubkeyAddrs) != consultN {
		log.Debugf("getETHAddrs result's len not enough")
		return shim.Error("getETHAddrs result's len not enough")
	}

	sigs := verifySigs(padderBytes, juryMsg, pubkeyAddrs, stub)
	if len(sigs) < consultM {
		log.Debugf("verifySigs result's len not enough")
		return shim.Error("verifySigs result's len not enough")
	}
	sigsStr := sigs[0]
	for i := 1; i < consultM; i++ {
		sigsStr = sigsStr + sigs[i]
	}
	sigHash := crypto.Keccak256([]byte(sigsStr))
	sigHashHex := fmt.Sprintf("%x", sigHash)
	log.Debugf("start consult sigHashHex %s", sigHashHex)

	//用签名列表的哈希协商交易签名，作适当安全防护
	txResult, err := consult(stub, []byte(sigHashHex), []byte("sigHash"))
	if err != nil {
		log.Debugf("consult sigHash failed: " + err.Error())
		return shim.Error("consult sigHash failed: " + err.Error())
	}
	var txJuryMsg []JuryMsgAddr
	err = json.Unmarshal(txResult, &txJuryMsg)
	if err != nil {
		log.Debugf("Unmarshal sigHash result failed: " + err.Error())
		return shim.Error("Unmarshal sigHash result failed: " + err.Error())
	}
	if len(txJuryMsg) < consultM {
		log.Debugf("RecvJury sigHash result's len not enough")
		return shim.Error("RecvJury sigHash result's len not enough")
	}
	//协商 保证协商一致后才写入签名结果
	txResult2, err := consult(stub, []byte(sigHashHex+"twice"), []byte("sigHash2"))
	if err != nil {
		log.Debugf("consult sigHash2 failed: " + err.Error())
		return shim.Error("consult sigHash2 failed: " + err.Error())
	}
	var txJuryMsg2 []JuryMsgAddr
	err = json.Unmarshal(txResult2, &txJuryMsg2)
	if err != nil {
		log.Debugf("Unmarshal sigHash2 result failed: " + err.Error())
		return shim.Error("Unmarshal sigHash2 result failed: " + err.Error())
	}
	if len(txJuryMsg2) < consultM {
		log.Debugf("RecvJury sigHash2 result's len not enough")
		return shim.Error("RecvJury sigHash2 result's len not enough")
	}

	//记录签名
	var withdraw Withdraw
	withdraw.EthAddr = ethAddr
	withdraw.EthAmount = ethAmount
	withdraw.EthFee = 0
	withdraw.Sigs = append(withdraw.Sigs, sigs[0:consultM]...)
	withdrawBytes, err := json.Marshal(withdraw)
	if err != nil {
		return shim.Error(err.Error())
	}
	err = stub.PutState(symbolsWithdraw+reqid, withdrawBytes)
	if err != nil {
		log.Debugf("save withdraw failed: " + err.Error())
		return shim.Error("save withdraw failed: " + err.Error())
	}

	return shim.Success(withdrawBytes)
}

func (p *ETHPort) Get(stub shim.ChaincodeStubInterface, key string) pb.Response {
	result, _ := stub.GetState(key)
	return shim.Success(result)
}

func main() {
	err := shim.Start(new(ETHPort))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}
}
