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

package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/palletone/go-palletone/contracts/syscontract"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/golang/protobuf/proto"
	"github.com/looplab/fsm"
	"github.com/palletone/go-palletone/common"
	"github.com/palletone/go-palletone/common/log"
	cfg "github.com/palletone/go-palletone/contracts/contractcfg"
	"github.com/palletone/go-palletone/contracts/outchain"
	"github.com/palletone/go-palletone/core/vmContractPub/ccprovider"
	//commonledger "github.com/palletone/go-palletone/core/vmContractPub/ledger"
	pb "github.com/palletone/go-palletone/core/vmContractPub/protos/peer"
	"github.com/palletone/go-palletone/core/vmContractPub/sysccprovider"
	"github.com/palletone/go-palletone/core/vmContractPub/util"
	"github.com/palletone/go-palletone/dag/modules"
	"github.com/palletone/go-palletone/dag/rwset"
	"github.com/palletone/go-palletone/vm/ccintf"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

const (
	createdstate     = "created"     //start state
	establishedstate = "established" //in: CREATED, rcv:  REGISTER, send: REGISTERED, INIT
	readystate       = "ready"       //in:ESTABLISHED,TRANSACTION, rcv:COMPLETED
	endstate         = "end"         //in:INIT,ESTABLISHED, rcv: error, terminate container

)

//var log = flogging.MustGetLogger("chaincode")

type transactionContext struct {
	chainID          string
	signedProp       *pb.PtnSignedProposal
	proposal         *pb.PtnProposal
	responseNotifier chan *pb.PtnChaincodeMessage

	// tracks open iterators used for range queries
	//queryIteratorMap    map[string]commonledger.ResultsIterator
	//pendingQueryResults map[string]*pendingQueryResult

	txsimulator rwset.TxSimulator
}

//type pendingQueryResult struct {
//	batch []*pb.QueryResultBytes
//	count int
//}

type nextStateInfo struct {
	msg      *pb.PtnChaincodeMessage
	sendToCC bool

	//the only time we need to send synchronously is
	//when launching the chaincode to take it to ready
	//state (look for the panic when sending serial)
	sendSync bool
}

type IAdapterJury interface {
	AdapterFunRequest(reqId common.Hash, contractId common.Address, msgType uint32, consultContent []byte, myAnswer []byte) ([]byte, error)
	AdapterFunResult(reqId common.Hash, contractId common.Address, msgType uint32, consultContent []byte, timeOut time.Duration) ([]byte, error)
	GetLocalJuryAddrs() []common.Address
}

// Handler responsible for management of Peer's side of chaincode stream
type Handler struct {
	sync.RWMutex
	//peer to shim grpc serializer. User only in serialSend
	serialLock  sync.Mutex
	ChatStream  ccintf.ChaincodeStream
	FSM         *fsm.FSM
	ChaincodeID *pb.PtnChaincodeID
	ccInstance  *sysccprovider.ChaincodeInstance

	chaincodeSupport *ChaincodeSupport
	registered       bool
	readyNotify      chan bool
	// Map of tx txid to either invoke tx. Each tx will be
	// added prior to execute and remove when done execute
	txCtxs map[string]*transactionContext

	txidMap map[string]bool

	// used to do Send after making sure the state transition is complete
	nextState chan *nextStateInfo

	aJury IAdapterJury
}

// HandleChaincodeStream Main loop for handling the associated Chaincode stream
func HandleChaincodeStream(chaincodeSupport *ChaincodeSupport, ctxt context.Context, stream ccintf.ChaincodeStream, jury IAdapterJury) error {
	deadline, ok := ctxt.Deadline()
	log.Debugf("Current context deadline = %s, ok = %v", deadline, ok)
	handler := newChaincodeSupportHandler(chaincodeSupport, stream, jury)
	err := handler.processStream()
	if err != nil {
		log.Debugf("handler process stream err: %s", err.Error())
	}
	return err
}

// Filter the Errors to allow NoTransitionError and CanceledError to not propagate for cases where embedded Err == nil
func filterError(errFromFSMEvent error) error {
	if errFromFSMEvent != nil {
		if noTransitionErr, ok := errFromFSMEvent.(*fsm.NoTransitionError); ok {
			if noTransitionErr.Err != nil {
				// Squash the NoTransitionError
				return errFromFSMEvent
			}
			log.Debugf("Ignoring NoTransitionError: %s", noTransitionErr)
		}
		if canceledErr, ok := errFromFSMEvent.(*fsm.CanceledError); ok {
			if canceledErr.Err != nil {
				// Squash the CanceledError
				return canceledErr
			}
			log.Debugf("Ignoring CanceledError: %s", canceledErr)
		}
	}
	return nil
}

func isCollectionSet(collection string) bool {
	//if collection == "" {
	//	return false
	//}
	//return true
	return collection != ""
}

func getChaincodeInstance(ccName string) *sysccprovider.ChaincodeInstance {
	b := []byte(ccName)
	ci := &sysccprovider.ChaincodeInstance{}

	//compute suffix (ie, chain name)
	i := bytes.IndexByte(b, '/')
	if i >= 0 {
		if i < len(b)-1 {
			ci.ChainID = string(b[i+1:])
		}
		b = b[:i]
	}

	//compute version
	i = bytes.IndexByte(b, ':')
	if i >= 0 {
		if i < len(b)-1 {
			ci.ChaincodeVersion = string(b[i+1:])
		}
		b = b[:i]
	}
	// remaining is the chaincode name
	ci.ChaincodeName = string(b)

	return ci
}

func shorttxid(txid string) string {
	if len(txid) < 8 {
		return txid
	}
	return txid[0:8]
}

func newChaincodeSupportHandler(chaincodeSupport *ChaincodeSupport, peerChatStream ccintf.ChaincodeStream, aJury IAdapterJury) *Handler {
	v := &Handler{
		ChatStream: peerChatStream,
	}
	v.chaincodeSupport = chaincodeSupport
	v.aJury = aJury
	//we want this to block
	v.nextState = make(chan *nextStateInfo)

	v.FSM = fsm.NewFSM(
		createdstate,
		fsm.Events{
			//Send REGISTERED, then, if deploy { trigger INIT(via INIT) } else { trigger READY(via COMPLETED) }
			{Name: pb.PtnChaincodeMessage_REGISTER.String(), Src: []string{createdstate}, Dst: establishedstate},
			{Name: pb.PtnChaincodeMessage_READY.String(), Src: []string{establishedstate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_PTN_PUT_STATE.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_OUTCHAIN_CALL.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_PTN_DEL_STATE.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_INVOKE_CHAINCODE.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_COMPLETED.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_PTN_GET_STATE.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_GET_TIMESTAMP.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_GET_STATE_BY_PREFIX.String(), Src: []string{readystate}, Dst: readystate},
			//{Name: pb.PtnChaincodeMessage_GET_HISTORY_FOR_KEY.String(), Src: []string{readystate}, Dst: readystate},
			//{Name: pb.PtnChaincodeMessage_QUERY_STATE_NEXT.String(), Src: []string{readystate}, Dst: readystate},
			//{Name: pb.PtnChaincodeMessage_QUERY_STATE_CLOSE.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_ERROR.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_RESPONSE.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_INIT.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_TRANSACTION.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_GET_SYSTEM_CONFIG_REQUEST.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_GET_CONTRACT_ALL_STATE.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_GET_TOKEN_BALANCE.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_GET_STABLE_TRANSACTION.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_GET_STABLE_UNIT.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_PAY_OUT_TOKEN.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_SUPPLY_TOKEN.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_DEFINE_TOKEN.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_GET_CERT_STATE.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_SEND_JURY.String(), Src: []string{readystate}, Dst: readystate},
			{Name: pb.PtnChaincodeMessage_RECV_JURY.String(), Src: []string{readystate}, Dst: readystate},
		},
		fsm.Callbacks{
			"before_" + pb.PtnChaincodeMessage_REGISTER.String():           func(e *fsm.Event) { v.beforeRegisterEvent(e, v.FSM.Current()) },
			"before_" + pb.PtnChaincodeMessage_COMPLETED.String():          func(e *fsm.Event) { v.beforeCompletedEvent(e) },
			"after_" + pb.PtnChaincodeMessage_PTN_GET_STATE.String():           func(e *fsm.Event) { v.afterGetState(e) },
			"after_" + pb.PtnChaincodeMessage_GET_TIMESTAMP.String():       func(e *fsm.Event) { v.afterGetTimestamp(e) },
			"after_" + pb.PtnChaincodeMessage_GET_STATE_BY_PREFIX.String(): func(e *fsm.Event) { v.afterGetStateByPrefix(e) },
			//"after_" + pb.PtnChaincodeMessage_GET_HISTORY_FOR_KEY.String():       func(e *fsm.Event) { v.afterGetHistoryForKey(e, v.FSM.Current()) },
			//"after_" + pb.PtnChaincodeMessage_QUERY_STATE_NEXT.String():          func(e *fsm.Event) { v.afterQueryStateNext(e, v.FSM.Current()) },
			//"after_" + pb.PtnChaincodeMessage_QUERY_STATE_CLOSE.String():         func(e *fsm.Event) { v.afterQueryStateClose(e, v.FSM.Current()) },
			"after_" + pb.PtnChaincodeMessage_PTN_PUT_STATE.String():                 func(e *fsm.Event) { v.enterBusyState(e, v.FSM.Current()) },
			"after_" + pb.PtnChaincodeMessage_OUTCHAIN_CALL.String():             func(e *fsm.Event) { v.enterOutChainCall(e) },
			"after_" + pb.PtnChaincodeMessage_SEND_JURY.String():                 func(e *fsm.Event) { v.enterSendJury(e) },
			"after_" + pb.PtnChaincodeMessage_RECV_JURY.String():                 func(e *fsm.Event) { v.enterRecvJury(e) },
			"after_" + pb.PtnChaincodeMessage_PTN_DEL_STATE.String():                 func(e *fsm.Event) { v.enterBusyState(e, v.FSM.Current()) },
			"after_" + pb.PtnChaincodeMessage_INVOKE_CHAINCODE.String():          func(e *fsm.Event) { v.enterBusyState(e, v.FSM.Current()) },
			"enter_" + establishedstate:                                       func(e *fsm.Event) { v.enterEstablishedState(e) },
			"enter_" + readystate:                                             func(e *fsm.Event) { v.enterReadyState(e, v.FSM.Current()) },
			"enter_" + endstate:                                               func(e *fsm.Event) { v.enterEndState(e, v.FSM.Current()) },
			"after_" + pb.PtnChaincodeMessage_GET_SYSTEM_CONFIG_REQUEST.String(): func(e *fsm.Event) { v.enterGetSystemConfig(e) },
			"after_" + pb.PtnChaincodeMessage_GET_CONTRACT_ALL_STATE.String():    func(e *fsm.Event) { v.enterGetContractAllState(e) },
			"after_" + pb.PtnChaincodeMessage_GET_TOKEN_BALANCE.String():         func(e *fsm.Event) { v.enterGetTokenBalance(e) },
			"after_" + pb.PtnChaincodeMessage_GET_STABLE_TRANSACTION.String():    func(e *fsm.Event) { v.enterGetStableTransactionByHash(e) },
			"after_" + pb.PtnChaincodeMessage_GET_STABLE_UNIT.String():           func(e *fsm.Event) { v.enterGetStableUnit(e) },
			"after_" + pb.PtnChaincodeMessage_PAY_OUT_TOKEN.String():             func(e *fsm.Event) { v.enterPayOutToken(e) },
			"after_" + pb.PtnChaincodeMessage_DEFINE_TOKEN.String():              func(e *fsm.Event) { v.enterDefineToken(e) },
			"after_" + pb.PtnChaincodeMessage_SUPPLY_TOKEN.String():              func(e *fsm.Event) { v.enterSupplyToken(e) },
			"after_" + pb.PtnChaincodeMessage_GET_CERT_STATE.String():            func(e *fsm.Event) { v.enterGetCertByID(e) },
		},
	)

	return v
}

//func (p *pendingQueryResult) cut() []*pb.QueryResultBytes {
//	batch := p.batch
//	p.batch = nil
//	p.count = 0
//	return batch
//}
//
//func (p *pendingQueryResult) add(queryResult commonledger.QueryResult) error {
//	queryResultBytes, err := proto.Marshal(queryResult.(proto.Message))
//	if err != nil {
//		log.Errorf("Failed to get encode query result as bytes")
//		return err
//	}
//	p.batch = append(p.batch, &pb.QueryResultBytes{ResultBytes: queryResultBytes})
//	p.count = len(p.batch)
//	return nil
//}

func (handler *Handler) enterGetSystemConfig(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	log.Debugf("[%s]Received %s, invoking get state from ledger", shorttxid(msg.Txid),
		pb.PtnChaincodeMessage_GET_SYSTEM_CONFIG_REQUEST)
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the afterGetState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode txid
		uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
		if !uniqueReq {
			// Drop this request
			log.Error("Another state request pending for this Txid. Cannot process.")
			return
		}

		var serialSendMsg *pb.PtnChaincodeMessage
		var txContext *transactionContext
		txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid,
			"[%s]No ledger context for GetState. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)
		if txContext == nil {
			return
		}
		defer func() {
			handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
			log.Debugf("[%s]handleEnterGetDepositConfig serial send %s",
				shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			handler.serialSendAsync(serialSendMsg, nil)
		}()
		keyForSystemConfig := &pb.PtnKeyForSystemConfig{}
		unmarshalErr := proto.Unmarshal(msg.Payload, keyForSystemConfig)
		if unmarshalErr != nil {
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(unmarshalErr.Error()),
				Txid: msg.Txid, ChannelId: msg.ChannelId}
			return
		}
		chaincodeID := handler.getCCRootName()
		log.Debugf("[%s] getting state for chaincode %s, channel %s", shorttxid(msg.Txid), chaincodeID,
			msg.ChannelId)

		//payloadBytes, err := txContext.txsimulator.GetChainParameters()
		payloadBytes, err := txContext.txsimulator.GetGlobalProp()
		if err != nil {
			log.Debugf("[%s]Got deposit configs. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(err.Error()),
				Txid: msg.Txid, ChannelId: msg.ChannelId}
			return
		}
		log.Debugf("[%s]Got deposit configs. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_RESPONSE)
		serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: payloadBytes, Txid: msg.Txid,
			ChannelId: msg.ChannelId}
	}()
}

func (handler *Handler) enterGetContractAllState(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	log.Debugf("[%s]Received %s, invoking GetContractAllState from ledger", shorttxid(msg.Txid),
		pb.PtnChaincodeMessage_GET_CONTRACT_ALL_STATE)
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the afterGetState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode txid
		uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
		if !uniqueReq {
			// Drop this request
			log.Error("Another state request pending for this Txid. Cannot process.")
			return
		}
		var serialSendMsg *pb.PtnChaincodeMessage
		var txContext *transactionContext
		txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid,
			"[%s]No ledger context for GetContractAllState. Sending %s", shorttxid(msg.Txid),
			pb.PtnChaincodeMessage_ERROR)
		defer func() {
			handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
			log.Debugf("[%s]enterGetContractAllState serial send %s",
				shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			handler.serialSendAsync(serialSendMsg, nil)
		}()
		if txContext == nil {
			return
		}
		chaincodeID := handler.getCCRootName()
		log.Debugf("[%s] getting contract all states for chaincode %s, channel %s", shorttxid(msg.Txid),
			chaincodeID, txContext.chainID)
		//返回 map[modules.StateVersion][]byte
		//contractAllStates := make(map[modules.StateVersion][]byte, 0)
		contractAllStates, err := txContext.txsimulator.GetAllStates(msg.ContractId, chaincodeID)

		//res, err := txContext.txsimulator.GetState(msg.ContractId, chaincodeID, getState.Key)
		if err != nil {
			// Send error msg back to chaincode. GetState will not trigger event
			payload := []byte(err.Error())
			log.Errorf("[%s]Failed to get contract all states(%s). Sending %s",
				shorttxid(msg.Txid), err, pb.PtnChaincodeMessage_ERROR)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else {
			res, err := json.Marshal(contractAllStates)
			if err != nil {
				serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(err.Error()), Txid: msg.Txid, ChannelId: msg.ChannelId}
				return
			}
			// Send response msg back to chaincode. GetState will not trigger event
			log.Debugf("[%s]Got contract all states. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: res, Txid: msg.Txid, ChannelId: msg.ChannelId}
		}
	}()
}

//gets chaincode instance from the canonical name of the chaincode.
//Called exactly once per chaincode when registering chaincode.
//This is needed for the "one-instance-per-chain" model when
//starting up the chaincode for each chain. It will still
//work for the "one-instance-for-all-chains" as the version
//and suffix will just be absent (also note that LSCC reserves
//"/:[]${}" as special chars mainly for such namespace uses)
func (handler *Handler) decomposeRegisteredName(cid *pb.PtnChaincodeID) {

	handler.ccInstance = getChaincodeInstance(cid.Name)
}

func (handler *Handler) getCCRootName() string {
	return handler.ccInstance.ChaincodeName
}

//serialSend serializes msgs so gRPC will be happy
func (handler *Handler) serialSend(msg *pb.PtnChaincodeMessage) error {
	handler.serialLock.Lock()
	defer handler.serialLock.Unlock()

	var err error
	if err = handler.ChatStream.Send(msg); err != nil {
		err = errors.WithMessage(err, fmt.Sprintf("[%s]Error sending %s", shorttxid(msg.Txid), msg.Type.String()))
		log.Errorf("%+v", err)
	}
	return err
}

//serialSendAsync serves the same purpose as serialSend (serialize msgs so gRPC will
//be happy). In addition, it is also asynchronous so send-remoterecv--localrecv loop
//can be nonblocking. Only errors need to be handled and these are handled by
//communication on supplied error channel. A typical use will be a non-blocking or
//nil channel
func (handler *Handler) serialSendAsync(msg *pb.PtnChaincodeMessage, errc chan error) {
	go func() {
		err := handler.serialSend(msg)
		if errc != nil {
			errc <- err
		}
	}()
}

//transaction context id should be composed of chainID and txid. While
//needed for CC-2-CC, it also allows users to concurrently send proposals
//with the same TXID to the SAME CC on multiple channels
func (handler *Handler) getTxCtxId(chainID string, txid string) string {
	return chainID + txid
}

func (handler *Handler) createTxContext(ctxt context.Context, chainID string, txid string, signedProp *pb.PtnSignedProposal, prop *pb.PtnProposal) (*transactionContext, error) {
	if handler.txCtxs == nil {
		return nil, errors.Errorf("cannot create notifier for txid: %s", txid)
	}
	handler.Lock()
	defer handler.Unlock()
	txCtxID := handler.getTxCtxId(chainID, txid)
	if handler.txCtxs[txCtxID] != nil {
		log.Debugf("createTxContext,  already exists, txCtxID[%s] ", txCtxID)
		return nil, errors.Errorf("txid: %s(%s) exists", txid, chainID)
	}
	txctx := &transactionContext{chainID: chainID, signedProp: signedProp,
		proposal: prop, responseNotifier: make(chan *pb.PtnChaincodeMessage, 1)}
	//glh
	//queryIteratorMap:    make(map[string]commonledger.ResultsIterator),
	//pendingQueryResults: make(map[string]*pendingQueryResult)}
	handler.txCtxs[txCtxID] = txctx
	log.Debugf("createTxContext, create txCtxID[%s]", txCtxID)
	//glh
	txctx.txsimulator = getTxSimulator(ctxt)
	//txctx.historyQueryExecutor = getHistoryQueryExecutor(ctxt)

	return txctx, nil
}

func (handler *Handler) getTxContext(chainID, txid string) *transactionContext {
	handler.Lock()
	defer handler.Unlock()
	txCtxID := handler.getTxCtxId(chainID, txid)
	return handler.txCtxs[txCtxID]
}

func (handler *Handler) deleteTxContext(chainID, txid string) {
	handler.Lock()
	defer handler.Unlock()
	txCtxID := handler.getTxCtxId(chainID, txid)
	if handler.txCtxs != nil {
		delete(handler.txCtxs, txCtxID)
	}
}

func (handler *Handler) deregister() {
	if handler.registered {
		handler.chaincodeSupport.deregisterHandler(handler)
	}
}

func (handler *Handler) triggerNextState(msg *pb.PtnChaincodeMessage, send bool) {
	//this will send Async
	handler.nextState <- &nextStateInfo{msg: msg, sendToCC: send, sendSync: false}
}

func (handler *Handler) triggerNextStateSync(msg *pb.PtnChaincodeMessage) {
	//this will send sync
	handler.nextState <- &nextStateInfo{msg: msg, sendToCC: true, sendSync: true}
}

func (handler *Handler) waitForKeepaliveTimer() <-chan time.Time {
	if handler.chaincodeSupport.keepalive > 0 {
		c := time.After(handler.chaincodeSupport.keepalive)
		return c
	}
	//no one will signal this channel, listener blocks forever
	c := make(chan time.Time, 1)
	return c
}

func (handler *Handler) processStream() error {
	defer handler.deregister()
	msgAvail := make(chan *pb.PtnChaincodeMessage)
	var nsInfo *nextStateInfo
	var in *pb.PtnChaincodeMessage
	var err error

	//recv is used to spin Recv routine after previous received msg
	//has been processed
	recv := true

	//catch send errors and bail now that sends aren't synchronous
	errc := make(chan error, 1)
	for {
		in = nil
		err = nil
		nsInfo = nil
		if recv {
			recv = false
			go func() {
				var in2 *pb.PtnChaincodeMessage
				in2, err = handler.ChatStream.Recv()
				msgAvail <- in2
			}()
		}
		select {
		case sendErr := <-errc:
			if sendErr != nil {
				log.Debugf("%+v", sendErr)
				return sendErr
			}
			//send was successful, just continue
			continue
		case in = <-msgAvail:
			// Defer the deregistering of the this handler.
			if err == io.EOF {
				err = errors.Wrapf(err, "received EOF, ending chaincode support stream")
				log.Debugf("%+v", err)
				return err
			} else if err != nil {
				//log.Errorf("Error handling chaincode support stream: %+v", err)
				log.Debugf("Error handling chaincode support stream: %+v", err)
				return err
			} else if in == nil {
				log.Debugf("in == nil")
				//err = errors.New("received nil message, ending chaincode support stream")
				//log.Debugf("%+v", err)
				return nil
			}
			log.Debugf("[%s]Received message %s from shim", shorttxid(in.Txid), in.Type.String())
			if in.Type.String() == pb.PtnChaincodeMessage_ERROR.String() {
				log.Errorf("Got error: %s", string(in.Payload))

			}

			// we can spin off another Recv again
			recv = true

			if in.Type == pb.PtnChaincodeMessage_KEEPALIVE {
				log.Debug("Received KEEPALIVE Response")
				// Received a keep alive message, we don't do anything with it for now
				// and it does not touch the state machine
				continue
			}
		case nsInfo = <-handler.nextState:
			in = nsInfo.msg
			if in == nil {
				err = errors.New("next state nil message, ending chaincode support stream")
				log.Debugf("%+v", err)
				return err
			}
			log.Debugf("[%s]Move state message %s", shorttxid(in.Txid), in.Type.String())
		case <-handler.waitForKeepaliveTimer():
			if handler.chaincodeSupport.keepalive <= 0 {
				log.Errorf("Invalid select: keepalive not on (keepalive=%d)", handler.chaincodeSupport.keepalive)
				continue
			}

			//if no error message from serialSend, KEEPALIVE happy, and don't care about error
			//(maybe it'll work later)
			handler.serialSendAsync(&pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_KEEPALIVE}, nil)
			continue
		}

		err = handler.handleMessage(in)
		if err != nil {
			err = errors.WithMessage(err, "error handling message, ending stream")
			log.Errorf("[%s] %+v", shorttxid(in.Txid), err)
			return err
		}

		if nsInfo != nil && nsInfo.sendToCC {
			log.Debugf("[%s]sending state message %s", shorttxid(in.Txid), in.Type.String())
			//ready messages are sent sync
			if nsInfo.sendSync {
				log.Debugf("send sync %v", nsInfo.sendSync)
				if in.Type.String() != pb.PtnChaincodeMessage_READY.String() {
					//  TODO
					return errors.Errorf("[%s]Sync send can only be for READY state %s\n", shorttxid(in.Txid), in.Type.String())
					//panic(fmt.Sprintf("[%s]Sync send can only be for READY state %s\n", shorttxid(in.Txid), in.Type.String()))
				}
				if err = handler.serialSend(in); err != nil {
					return errors.WithMessage(err, fmt.Sprintf("[%s]error sending ready  message, ending stream:", shorttxid(in.Txid)))
				}
			} else {
				log.Debugf("send async")
				//if error bail in select
				handler.serialSendAsync(in, errc)
			}
		}
	}
}

func (handler *Handler) createTXIDEntry(channelID, txid string) bool {
	if handler.txidMap == nil {
		return false
	}
	handler.Lock()
	defer handler.Unlock()
	txCtxID := handler.getTxCtxId(channelID, txid)
	if handler.txidMap[txCtxID] {
		return false
	}
	handler.txidMap[txCtxID] = true
	return handler.txidMap[txCtxID]
}

func (handler *Handler) deleteTXIDEntry(channelID, txid string) {
	handler.Lock()
	defer handler.Unlock()
	txCtxID := handler.getTxCtxId(channelID, txid)
	if handler.txidMap != nil {
		delete(handler.txidMap, txCtxID)
	} else {
		log.Debugf("TXID %s not found!", txCtxID)
	}
}

func (handler *Handler) notifyDuringStartup(val bool) {
	//if USER_RUNS_CC readyNotify will be nil
	if handler.readyNotify != nil {
		log.Debug("Notifying during startup")
		handler.readyNotify <- val
	} else {
		log.Debug("nothing to notify (dev mode ?)")
		//In theory, we don't even need a devmode flag in the peer anymore
		//as the chaincode is brought up without any context (ledger context
		//in particular). What this means is we can have - in theory - a nondev
		//environment where we can attach a chaincode manually. This could be
		//useful .... but for now lets just be conservative and allow manual
		//chaincode only in dev mode (ie, peer started with --peer-chaincodedev=true)
		if handler.chaincodeSupport.userRunsCC {
			if val {
				log.Debug("sending READY")
				ccMsg := &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_READY}
				go handler.triggerNextState(ccMsg, true)
			} else {
				log.Errorf("Error during startup .. not sending READY")
			}
		} else {
			log.Warn("trying to manually run chaincode when not in devmode ?")
		}
	}
}

// beforeRegisterEvent is invoked when chaincode tries to register.
func (handler *Handler) beforeRegisterEvent(e *fsm.Event, state string) {
	log.Debugf("Received %s in state %s", e.Event, state)
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	chaincodeID := &pb.PtnChaincodeID{}
	err := proto.Unmarshal(msg.Payload, chaincodeID)
	if err != nil {
		e.Cancel(errors.Wrap(err, fmt.Sprintf("error in received %s, could NOT unmarshal registration info", pb.PtnChaincodeMessage_REGISTER)))
		return
	}

	// Now register with the chaincodeSupport
	handler.ChaincodeID = chaincodeID
	err = handler.chaincodeSupport.registerHandler(handler)
	if err != nil {
		e.Cancel(errors.New(err.Error()))
		handler.notifyDuringStartup(false)
		return
	}

	//get the component parts so we can use the root chaincode
	//name in keys
	handler.decomposeRegisteredName(handler.ChaincodeID)

	log.Debugf("Got %s for chaincodeID = %s, sending back %s", e.Event, chaincodeID, pb.PtnChaincodeMessage_REGISTERED)
	if err := handler.serialSend(&pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_REGISTERED}); err != nil {
		e.Cancel(errors.WithMessage(err, fmt.Sprintf("error sending %s", pb.PtnChaincodeMessage_REGISTERED)))
		handler.notifyDuringStartup(false)
		return
	}
}

func (handler *Handler) notify(msg *pb.PtnChaincodeMessage) {
	handler.Lock()
	defer handler.Unlock()
	txCtxID := handler.getTxCtxId(msg.ChannelId, msg.Txid)
	tctx := handler.txCtxs[txCtxID]
	if tctx == nil {
		log.Debugf("notifier Txid:%s, channelID:%s does not exist", msg.Txid, msg.ChannelId)
	} else {
		log.Debugf("notifying Txid:%s, channelID:%s", msg.Txid, msg.ChannelId)
		tctx.responseNotifier <- msg

		// clean up queryIteratorMap
		//for _, v := range tctx.queryIteratorMap {
		//	v.Close()
		//}
	}
}

// beforeCompletedEvent is invoked when chaincode has completed execution of init, invoke.
func (handler *Handler) beforeCompletedEvent(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	// Notify on channel once into READY state
	log.Debugf("[%s]beforeCompleted - not in ready state will notify when in readystate", shorttxid(msg.Txid))
}
func (handler *Handler) afterGetStateByPrefix(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	log.Debugf("[%s]Received %s, invoking get state from ledger", shorttxid(msg.Txid), pb.PtnChaincodeMessage_PTN_GET_STATE)

	// Query ledger for state
	handler.handleGetStateByPrefix(msg)
}

func (handler *Handler) handleGetStateByPrefix(msg *pb.PtnChaincodeMessage) {
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the afterGetState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode txid
		uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
		if !uniqueReq {
			// Drop this request
			log.Error("Another state request pending for this Txid. Cannot process.")
			return
		}

		var serialSendMsg *pb.PtnChaincodeMessage
		var txContext *transactionContext
		txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid,
			"[%s]No ledger context for GetState. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)

		defer func() {
			handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
			//if serialSendMsg != nil {
			//	log.Debugf("[%s]handleGetState serial send %s",
			//		shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			//	handler.serialSendAsync(serialSendMsg, nil)
			//}
			log.Debugf("[%s]handleGetState serial send %s",
				shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			handler.serialSendAsync(serialSendMsg, nil)
		}()

		if txContext == nil {
			return
			//serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte("No ledger context for GetState. Sending error"), Txid: msg.Txid, ChannelId: msg.ChannelId}
			//return
		}
		getState := &pb.PtnGetStateByPrefix{}
		unmarshalErr := proto.Unmarshal(msg.Payload, getState)
		if unmarshalErr != nil {
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(unmarshalErr.Error()), Txid: msg.Txid, ChannelId: msg.ChannelId}
			return
		}
		chaincodeID := handler.getCCRootName()
		log.Debugf("[%s] getting state for chaincode %s, prefix %s, channel %s",
			shorttxid(msg.Txid), chaincodeID, getState.Prefix, txContext.chainID)

		rows, err := txContext.txsimulator.GetStatesByPrefix(msg.ContractId, chaincodeID, getState.Prefix)
		if err != nil {
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(err.Error()), Txid: msg.Txid, ChannelId: msg.ChannelId}
			return
		}
		res, err := json.Marshal(rows)
		//if txContext.txsimulator != nil {
		//	res, err = txContext.txsimulator.GetState(msg.ContractId, chaincodeID, getState.Key)
		//}
		if err != nil {
			// Send error msg back to chaincode. GetState will not trigger event
			payload := []byte(err.Error())
			log.Errorf("[%s]Failed to get chaincode state(%s). Sending %s",
				shorttxid(msg.Txid), err, pb.PtnChaincodeMessage_ERROR)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else if res == nil {
			//The state object being requested does not exist
			log.Debugf("[%s]No state associated with key: %s. Sending %s with an empty payload",
				shorttxid(msg.Txid), getState.Prefix, pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: res, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else {
			// Send response msg back to chaincode. GetState will not trigger event
			log.Debugf("[%s]Got state. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: res, Txid: msg.Txid, ChannelId: msg.ChannelId}
		}
	}()
}

// afterGetState handles a GET_STATE request from the chaincode.
func (handler *Handler) afterGetState(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	log.Debugf("[%s]Received %s, invoking get state from ledger", shorttxid(msg.Txid), pb.PtnChaincodeMessage_PTN_GET_STATE)

	// Query ledger for state
	handler.handleGetState(msg)
}
func (handler *Handler) afterGetTimestamp(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	log.Debugf("[%s]Received %s, invoking get state from ledger", shorttxid(msg.Txid), pb.PtnChaincodeMessage_GET_TIMESTAMP)

	// Query ledger for state
	handler.handleGetTimestamp(msg)
}
func (handler *Handler) enterGetTokenBalance(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	log.Debugf("[%s]Received %s, invoking get state from ledger", shorttxid(msg.Txid), pb.PtnChaincodeMessage_GET_TOKEN_BALANCE)

	// Query ledger for state
	handler.handleGetTokenBalance(msg)
}

// Handles query to ledger to get state
func (handler *Handler) handleGetTokenBalance(msg *pb.PtnChaincodeMessage) {
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the afterGetState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode txid
		uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
		if !uniqueReq {
			// Drop this request
			log.Error("Another state request pending for this Txid. Cannot process.")
			return
		}

		var serialSendMsg *pb.PtnChaincodeMessage
		var txContext *transactionContext
		txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid,
			"[%s]No ledger context for GetTokenBalance. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)

		defer func() {
			handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
			//if serialSendMsg != nil {
			//	log.Debugf("[%s]handleGetState serial send %s",
			//		shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			//	handler.serialSendAsync(serialSendMsg, nil)
			//}
			log.Debugf("[%s]handleGetTokenBalance serial send %s",
				shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			handler.serialSendAsync(serialSendMsg, nil)
		}()

		if txContext == nil {
			return
			//serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte("No ledger context for GetState. Sending error"), Txid: msg.Txid, ChannelId: msg.ChannelId}
			//return
		}
		getBalance := &pb.PtnGetTokenBalance{}
		unmarshalErr := proto.Unmarshal(msg.Payload, getBalance)
		if unmarshalErr != nil {
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(unmarshalErr.Error()), Txid: msg.Txid, ChannelId: msg.ChannelId}
			return
		}
		chaincodeID := handler.getCCRootName()
		log.Debugf("[%s] getting balance for chaincode %s, asset: %s,address: %s, channel %s",
			shorttxid(msg.Txid), chaincodeID, getBalance.Asset, getBalance.Address, txContext.chainID)
		var balance map[modules.Asset]uint64
		var err error
		addr := getBalance.Address
		if len(addr) == 0 { //Get current contract address balance
			addr = common.NewAddress(msg.ContractId, common.ContractHash).String()
			log.Debugf("Address is nil, use contract id:%x, address:%s", msg.ContractId, addr)
		}

		address, err := common.StringToAddress(addr)
		if err != nil {
			log.Error(err.Error())
			return
		}

		if len(getBalance.Asset) > 0 {
			asset := &modules.Asset{}
			asset.SetString(getBalance.Asset)
			balance, err = txContext.txsimulator.GetTokenBalance(chaincodeID, address, asset)
		} else { // get all token type balance
			balance, err = txContext.txsimulator.GetTokenBalance(chaincodeID, address, nil)

		}

		if err != nil {
			// Send error msg back to chaincode. GetState will not trigger event
			payload := []byte(err.Error())
			log.Errorf("[%s]Failed to get chaincode state(%s). Sending %s",
				shorttxid(msg.Txid), err, pb.PtnChaincodeMessage_ERROR)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else if balance == nil {
			//The state object being requested does not exist
			log.Debugf("[%s]No state associated with key: %s. Sending %s with an empty payload",
				shorttxid(msg.Txid), getBalance.Asset, pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: nil, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else {
			// Send response msg back to chaincode. GetState will not trigger event
			result := []*modules.InvokeTokens{}
			for asset, amt := range balance {
				asset := asset
				result = append(result, &modules.InvokeTokens{Amount: amt, Asset: &asset, Address: address.String()})
			}
			res, _ := rlp.EncodeToBytes(result)
			log.Debugf("[%s]Got state. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: res, Txid: msg.Txid, ChannelId: msg.ChannelId}
		}
	}()
}

func (handler *Handler) enterGetStableTransactionByHash(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	log.Debugf("[%s]Received %s, invoking get state from ledger", shorttxid(msg.Txid), pb.PtnChaincodeMessage_GET_STABLE_TRANSACTION)

	// Query ledger for state
	handler.handleGetStableTransactionByHash(msg)
}

// Handles query to ledger to get state
func (handler *Handler) handleGetStableTransactionByHash(msg *pb.PtnChaincodeMessage) {
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the afterGetState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode txid
		uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
		if !uniqueReq {
			// Drop this request
			log.Error("Another state request pending for this Txid. Cannot process.")
			return
		}

		var serialSendMsg *pb.PtnChaincodeMessage
		var txContext *transactionContext
		txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid,
			"[%s]No ledger context for handleGetStableTransactionByHash. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)

		defer func() {
			handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
			//if serialSendMsg != nil {
			//	log.Debugf("[%s]handleGetState serial send %s",
			//		shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			//	handler.serialSendAsync(serialSendMsg, nil)
			//}
			log.Debugf("[%s]handleGetStableTransactionByHash serial send %s",
				shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			handler.serialSendAsync(serialSendMsg, nil)
		}()

		if txContext == nil {
			return
			//serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte("No ledger context for GetState. Sending error"), Txid: msg.Txid, ChannelId: msg.ChannelId}
			//return
		}
		getStableTX := &pb.PtnGetStableTransaction{}
		unmarshalErr := proto.Unmarshal(msg.Payload, getStableTX)
		if unmarshalErr != nil {
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(unmarshalErr.Error()), Txid: msg.Txid, ChannelId: msg.ChannelId}
			return
		}
		chaincodeID := handler.getCCRootName()
		log.Debugf("[%s] GetStableTransaction for chaincode %s, TxHash: %s, channel %s",
			shorttxid(msg.Txid), chaincodeID, getStableTX.TxHash, txContext.chainID)

		txHash := common.Hash{}
		err := txHash.SetHexString(getStableTX.TxHash)
		if err != nil {
			log.Error(err.Error())
			return
		}
		tx, err := txContext.txsimulator.GetStableTransactionByHash(chaincodeID, txHash)

		if err != nil {
			// Send error msg back to chaincode. GetState will not trigger event
			payload := []byte(err.Error())
			log.Errorf("[%s]Failed to get chaincode state(%s). Sending %s",
				shorttxid(msg.Txid), err, pb.PtnChaincodeMessage_ERROR)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else if tx == nil {
			//The state object being requested does not exist
			log.Debugf("[%s]No state associated with key: %s. Sending %s with an empty payload",
				shorttxid(msg.Txid), getStableTX.TxHash, pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: nil, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else {
			// Send response msg back to chaincode. GetState will not trigger event
			res, _ := rlp.EncodeToBytes(tx)
			log.Debugf("[%s]Got state. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: res, Txid: msg.Txid, ChannelId: msg.ChannelId}
		}
	}()
}

func (handler *Handler) enterGetStableUnit(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	log.Debugf("[%s]Received %s, invoking get state from ledger", shorttxid(msg.Txid), pb.PtnChaincodeMessage_GET_STABLE_UNIT)

	// Query ledger for state
	handler.handleGetStableUnit(msg)
}

// Handles query to ledger to get state
func (handler *Handler) handleGetStableUnit(msg *pb.PtnChaincodeMessage) {
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the afterGetState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode txid
		uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
		if !uniqueReq {
			// Drop this request
			log.Error("Another state request pending for this Txid. Cannot process.")
			return
		}

		var serialSendMsg *pb.PtnChaincodeMessage
		var txContext *transactionContext
		txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid,
			"[%s]No ledger context for handleGetStableTransactionByHash. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)

		defer func() {
			handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
			//if serialSendMsg != nil {
			//	log.Debugf("[%s]handleGetState serial send %s",
			//		shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			//	handler.serialSendAsync(serialSendMsg, nil)
			//}
			log.Debugf("[%s]handleGetStableTransactionByHash serial send %s",
				shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			handler.serialSendAsync(serialSendMsg, nil)
		}()

		if txContext == nil {
			return
			//serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte("No ledger context for GetState. Sending error"), Txid: msg.Txid, ChannelId: msg.ChannelId}
			//return
		}
		getStableUnit := &pb.PtnGetStableUnit{}
		unmarshalErr := proto.Unmarshal(msg.Payload, getStableUnit)
		if unmarshalErr != nil {
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(unmarshalErr.Error()), Txid: msg.Txid, ChannelId: msg.ChannelId}
			return
		}
		chaincodeID := handler.getCCRootName()
		log.Debugf("[%s] GetStableTransaction for chaincode %s, UnitHash: %s, UnitNumber: %d, channel %s",
			shorttxid(msg.Txid), chaincodeID, getStableUnit.UnitHash, getStableUnit.UnitNumber, txContext.chainID)

		unitHash := common.Hash{}
		if getStableUnit.UnitHash != "" {
			err := unitHash.SetHexString(getStableUnit.UnitHash)
			if err != nil {
				log.Error(err.Error())
				return
			}
		}

		unit, err := txContext.txsimulator.GetStableUnit(chaincodeID, unitHash, getStableUnit.UnitNumber)

		if err != nil {
			// Send error msg back to chaincode. GetState will not trigger event
			payload := []byte(err.Error())
			log.Errorf("[%s]Failed to get chaincode state(%s). Sending %s",
				shorttxid(msg.Txid), err, pb.PtnChaincodeMessage_ERROR)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else if unit == nil {
			//The state object being requested does not exist
			log.Debugf("[%s]No state associated with key: %s. Sending %s with an empty payload",
				shorttxid(msg.Txid), getStableUnit.UnitHash, pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: nil, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else {
			// Send response msg back to chaincode. GetState will not trigger event
			res, _ := rlp.EncodeToBytes(unit)
			log.Debugf("[%s]Got state. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: res, Txid: msg.Txid, ChannelId: msg.ChannelId}
		}
	}()
}

func (handler *Handler) enterPayOutToken(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	log.Debugf("[%s]Received %s, invoking get state from ledger", shorttxid(msg.Txid), pb.PtnChaincodeMessage_PAY_OUT_TOKEN)

	handler.handlePayOutToken(msg)
}
func (handler *Handler) handlePayOutToken(msg *pb.PtnChaincodeMessage) {
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the afterGetState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode txid
		uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
		if !uniqueReq {
			// Drop this request
			log.Error("Another state request pending for this Txid. Cannot process.")
			return
		}

		var serialSendMsg *pb.PtnChaincodeMessage
		var txContext *transactionContext
		txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid,
			"[%s]No ledger context for handlePayOutToken. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)

		defer func() {
			handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
			//if serialSendMsg != nil {
			//	log.Debugf("[%s]handleGetState serial send %s",
			//		shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			//	handler.serialSendAsync(serialSendMsg, nil)
			//}
			log.Debugf("[%s]handlePayOutToken serial send %s",
				shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			handler.serialSendAsync(serialSendMsg, nil)
		}()

		if txContext == nil {
			return
			//serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte("No ledger context for GetState. Sending error"), Txid: msg.Txid, ChannelId: msg.ChannelId}
			//return
		}
		payout := &pb.PtnPayOutToken{}
		unmarshalErr := proto.Unmarshal(msg.Payload, payout)
		if unmarshalErr != nil {
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(unmarshalErr.Error()), Txid: msg.Txid, ChannelId: msg.ChannelId}
			return
		}
		//
		asset := &modules.Asset{}
		asset.SetBytes(payout.Asset)
		chaincodeID := handler.getCCRootName()
		log.Debugf("[%s] getting balance for chaincode %s, key %#v, channel %s",
			shorttxid(msg.Txid), chaincodeID, asset.String(), txContext.chainID)
		err := txContext.txsimulator.PayOutToken(chaincodeID, payout.Address, asset, payout.Amount, payout.Locktime)
		if err != nil {
			// Send error msg back to chaincode. GetState will not trigger event
			payload := []byte(err.Error())
			log.Errorf("[%s]Failed to get chaincode state(%s). Sending %s",
				shorttxid(msg.Txid), err, pb.PtnChaincodeMessage_ERROR)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else {
			// Send response msg back to chaincode. GetState will not trigger event
			log.Debugf("[%s]Got state. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: nil, Txid: msg.Txid, ChannelId: msg.ChannelId}
		}
	}()
}

func (handler *Handler) enterDefineToken(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	log.Debugf("[%s]Received %s, invoking get state from ledger", shorttxid(msg.Txid), pb.PtnChaincodeMessage_DEFINE_TOKEN)

	handler.handleDefineToken(msg)
}
func (handler *Handler) handleDefineToken(msg *pb.PtnChaincodeMessage) {
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the afterGetState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode txid
		uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
		if !uniqueReq {
			// Drop this request
			log.Error("Another state request pending for this Txid. Cannot process.")
			return
		}

		var serialSendMsg *pb.PtnChaincodeMessage
		var txContext *transactionContext
		txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid,
			"[%s]No ledger context for handlePayOutToken. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)

		defer func() {
			handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
			//if serialSendMsg != nil {
			//	log.Debugf("[%s]handleGetState serial send %s",
			//		shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			//	handler.serialSendAsync(serialSendMsg, nil)
			//}
			log.Debugf("[%s]handlePayOutToken serial send %s",
				shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			handler.serialSendAsync(serialSendMsg, nil)
		}()

		if txContext == nil {
			return
			//serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte("No ledger context for GetState. Sending error"), Txid: msg.Txid, ChannelId: msg.ChannelId}
			//return
		}
		payout := &pb.PtnDefineToken{}
		unmarshalErr := proto.Unmarshal(msg.Payload, payout)
		if unmarshalErr != nil {
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(unmarshalErr.Error()), Txid: msg.Txid, ChannelId: msg.ChannelId}
			return
		}
		chaincodeID := handler.getCCRootName()
		log.Debugf("[%s] define token for chaincode %s, token define %s, channel %s",
			shorttxid(msg.Txid), chaincodeID, string(payout.Define), txContext.chainID)

		err := txContext.txsimulator.DefineToken(chaincodeID, payout.TokenType, payout.Define, payout.Creator)

		if err != nil {
			// Send error msg back to chaincode. GetState will not trigger event
			payload := []byte(err.Error())
			log.Errorf("[%s]Failed to get chaincode state(%s). Sending %s",
				shorttxid(msg.Txid), err, pb.PtnChaincodeMessage_ERROR)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else {
			// Send response msg back to chaincode. GetState will not trigger event
			log.Debugf("[%s]Got state. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: nil, Txid: msg.Txid, ChannelId: msg.ChannelId}
		}
	}()
}

func (handler *Handler) enterSupplyToken(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	log.Debugf("[%s]Received %s, invoking get state from ledger", shorttxid(msg.Txid), pb.PtnChaincodeMessage_SUPPLY_TOKEN)

	handler.handleSupplyToken(msg)
}
func (handler *Handler) handleSupplyToken(msg *pb.PtnChaincodeMessage) {
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the afterGetState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode txid
		uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
		if !uniqueReq {
			// Drop this request
			log.Error("Another state request pending for this Txid. Cannot process.")
			return
		}

		var serialSendMsg *pb.PtnChaincodeMessage
		var txContext *transactionContext
		txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid,
			"[%s]No ledger context for handlePayOutToken. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)

		defer func() {
			handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
			//if serialSendMsg != nil {
			//	log.Debugf("[%s]handleGetState serial send %s",
			//		shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			//	handler.serialSendAsync(serialSendMsg, nil)
			//}
			log.Debugf("[%s]handlePayOutToken serial send %s",
				shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			handler.serialSendAsync(serialSendMsg, nil)
		}()

		if txContext == nil {
			return
			//serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte("No ledger context for GetState. Sending error"), Txid: msg.Txid, ChannelId: msg.ChannelId}
			//return
		}
		payout := &pb.PtnSupplyToken{}
		unmarshalErr := proto.Unmarshal(msg.Payload, payout)
		if unmarshalErr != nil {
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(unmarshalErr.Error()), Txid: msg.Txid, ChannelId: msg.ChannelId}
			return
		}
		chaincodeID := handler.getCCRootName()
		log.Debugf("[%s] define token for chaincode %s, token asset id: %X, channel %s",
			shorttxid(msg.Txid), chaincodeID, payout.AssetId, txContext.chainID)

		err := txContext.txsimulator.SupplyToken(chaincodeID, payout.AssetId, payout.UniqueId, payout.Amount, payout.Creator)

		if err != nil {
			// Send error msg back to chaincode. GetState will not trigger event
			payload := []byte(err.Error())
			log.Errorf("[%s]Failed to get chaincode state(%s). Sending %s",
				shorttxid(msg.Txid), err, pb.PtnChaincodeMessage_ERROR)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else {
			// Send response msg back to chaincode. GetState will not trigger event
			log.Debugf("[%s]Got state. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: nil, Txid: msg.Txid, ChannelId: msg.ChannelId}
		}
	}()
}

// is this a txid for which there is a valid txsim
func (handler *Handler) isValidTxSim(channelID string, txid string, fmtStr string, args ...interface{}) (*transactionContext, *pb.PtnChaincodeMessage) {
	txContext := handler.getTxContext(channelID, txid)
	//glh
	/*
		if txContext == nil || txContext.txsimulator == nil {
			// Send error msg back to chaincode. No ledger context
			errStr := fmt.Sprintf(fmtStr, args...)
			log.Errorf(errStr)
			return nil, &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(errStr), Txid: txid, ChannelId: channelID}
		}
	*/
	if txContext == nil || txContext.txsimulator == nil {
		// Send error msg back to chaincode. No ledger context
		errStr := fmt.Sprintf(fmtStr, args...)
		log.Errorf(errStr)
		return nil, &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(errStr), Txid: txid, ChannelId: channelID}
	}
	return txContext, nil
}

// Handles query to ledger to get state
func (handler *Handler) handleGetState(msg *pb.PtnChaincodeMessage) {
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the afterGetState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode txid
		uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
		if !uniqueReq {
			// Drop this request
			log.Error("Another state request pending for this Txid. Cannot process.")
			return
		}

		var serialSendMsg *pb.PtnChaincodeMessage
		var txContext *transactionContext
		txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid,
			"[%s]No ledger context for GetState. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)

		defer func() {
			handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
			//if serialSendMsg != nil {
			//	log.Debugf("[%s]handleGetState serial send %s",
			//		shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			//	handler.serialSendAsync(serialSendMsg, nil)
			//}
			log.Debugf("[%s]handleGetState serial send %s",
				shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			handler.serialSendAsync(serialSendMsg, nil)
		}()

		if txContext == nil {
			return
			//serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte("No ledger context for GetState. Sending error"), Txid: msg.Txid, ChannelId: msg.ChannelId}
			//return
		}
		getState := &pb.PtnGetState{}
		unmarshalErr := proto.Unmarshal(msg.Payload, getState)
		if unmarshalErr != nil {
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(unmarshalErr.Error()), Txid: msg.Txid, ChannelId: msg.ChannelId}
			return
		}
		chaincodeID := handler.getCCRootName()
		log.Debugf("[%s] getting state for chaincode %s, key %s, channel %s",
			shorttxid(msg.Txid), chaincodeID, getState.Key, txContext.chainID)

		var res []byte
		var err error
		if isCollectionSet(getState.Collection) {
			//glh
			//res, err = txContext.txsimulator.GetPrivateData(chaincodeID, getState.Collection, getState.Key)
		} else {
			res, err = txContext.txsimulator.GetState(msg.ContractId, chaincodeID, getState.Key)
			//glh
			//res, err = txContext.txsimulator.GetState(chaincodeID, getState.Key)
		}
		//if txContext.txsimulator != nil {
		//	res, err = txContext.txsimulator.GetState(msg.ContractId, chaincodeID, getState.Key)
		//}
		if err != nil {
			// Send error msg back to chaincode. GetState will not trigger event
			payload := []byte(err.Error())
			log.Errorf("[%s]Failed to get chaincode state(%s). Sending %s",
				shorttxid(msg.Txid), err, pb.PtnChaincodeMessage_ERROR)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else if res == nil {
			//The state object being requested does not exist
			log.Debugf("[%s]No state associated with key: %s. Sending %s with an empty payload",
				shorttxid(msg.Txid), getState.Key, pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: res, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else {
			// Send response msg back to chaincode. GetState will not trigger event
			log.Debugf("[%s]Got state. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: res, Txid: msg.Txid, ChannelId: msg.ChannelId}
		}
	}()
}

// Handles query to ledger to get state
func (handler *Handler) handleGetTimestamp(msg *pb.PtnChaincodeMessage) {
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the afterGetState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode txid
		uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
		if !uniqueReq {
			// Drop this request
			log.Error("Another state request pending for this Txid. Cannot process.")
			return
		}

		var serialSendMsg *pb.PtnChaincodeMessage
		var txContext *transactionContext
		txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid,
			"[%s]No ledger context for GetState. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)

		defer func() {
			handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
			//if serialSendMsg != nil {
			//	log.Debugf("[%s]handleGetState serial send %s",
			//		shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			//	handler.serialSendAsync(serialSendMsg, nil)
			//}
			log.Debugf("[%s]handleGetState serial send %s",
				shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			handler.serialSendAsync(serialSendMsg, nil)
		}()

		if txContext == nil {
			return
			//serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte("No ledger context for GetState. Sending error"), Txid: msg.Txid, ChannelId: msg.ChannelId}
			//return
		}
		getTimestamp := &pb.PtnGetTimestamp{}
		unmarshalErr := proto.Unmarshal(msg.Payload, getTimestamp)
		if unmarshalErr != nil {
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(unmarshalErr.Error()), Txid: msg.Txid, ChannelId: msg.ChannelId}
			return
		}
		chaincodeID := handler.getCCRootName()
		log.Debugf("[%s] getting state for chaincode %s, key %d, channel %s",
			shorttxid(msg.Txid), chaincodeID, getTimestamp.RangeNumber, txContext.chainID)

		var res []byte
		var err error
		if isCollectionSet(getTimestamp.Collection) {
			//glh
			//res, err = txContext.txsimulator.GetPrivateData(chaincodeID, getTimestamp.Collection, getTimestamp.Key)
		} else {
			res, err = txContext.txsimulator.GetTimestamp(chaincodeID, getTimestamp.RangeNumber)
			//glh
			//res, err = txContext.txsimulator.GetState(chaincodeID, getTimestamp.Key)
		}
		//if txContext.txsimulator != nil {
		//	res, err = txContext.txsimulator.GetState(msg.ContractId, chaincodeID, getTimestamp.Key)
		//}
		if err != nil {
			// Send error msg back to chaincode. GetState will not trigger event
			payload := []byte(err.Error())
			log.Errorf("[%s]Failed to get chaincode state(%s). Sending %s",
				shorttxid(msg.Txid), err, pb.PtnChaincodeMessage_ERROR)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else if res == nil {
			//The state object being requested does not exist
			log.Debugf("[%s]No state associated with key: %d. Sending %s with an empty payload",
				shorttxid(msg.Txid), getTimestamp.RangeNumber, pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: res, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else {
			// Send response msg back to chaincode. GetState will not trigger event
			log.Debugf("[%s]Got state. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: res, Txid: msg.Txid, ChannelId: msg.ChannelId}
		}
	}()
}

func (handler *Handler) getTxContextForMessage(channelID string, txid string, msgType string, payload []byte, fmtStr string, args ...interface{}) (*transactionContext, *pb.PtnChaincodeMessage) {
	//if we have a channelID, just get the txsim from isValidTxSim
	//if this is NOT an INVOKE_CHAINCODE, then let isValidTxSim handle retrieving the txContext
	if channelID != "" || msgType != pb.PtnChaincodeMessage_INVOKE_CHAINCODE.String() {
		return handler.isValidTxSim(channelID, txid, fmtStr, args...)
	}

	var calledCcIns *sysccprovider.ChaincodeInstance
	var txContext *transactionContext
	var triggerNextStateMsg *pb.PtnChaincodeMessage

	// prepare to get isscc (only for INVOKE_CHAINCODE, any other msgType will always call isValidTxSim to get the tx context)

	chaincodeSpec := &pb.PtnChaincodeSpec{}
	unmarshalErr := proto.Unmarshal(payload, chaincodeSpec)
	if unmarshalErr != nil {
		errStr := fmt.Sprintf("[%s]Unable to decipher payload. Sending %s", shorttxid(txid), pb.PtnChaincodeMessage_ERROR)
		triggerNextStateMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(errStr), Txid: txid}
		return nil, triggerNextStateMsg
	}
	// Get the chaincodeID to invoke. The chaincodeID to be called may
	// contain composite info like "chaincode-name:version/channel-name"
	// We are not using version now but default to the latest
	if calledCcIns = getChaincodeInstance(chaincodeSpec.ChaincodeId.Name); calledCcIns == nil {
		triggerNextStateMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte("could not get chaincode name for INVOKE_CHAINCODE"), Txid: txid}
		return nil, triggerNextStateMsg
	}

	//   If calledCcIns is not an SCC, isValidTxSim should be called which will return an err.
	//   We do not want to propagate calls to user CCs when the original call was to a SCC
	//   without a channel context (ie, no ledger context).
	if isscc := sysccprovider.GetSystemChaincodeProvider().IsSysCC(calledCcIns.ChaincodeName); !isscc {
		// normal path - UCC invocation with an empty ("") channel: isValidTxSim will return an error
		return handler.isValidTxSim("", txid, fmtStr, args...)
	}

	// Calling SCC without a  ChainID, then the assumption this is an external SCC called by the client (special case) and no UCC involved,
	// so no Transaction Simulator validation needed as there are no commits to the ledger, get the txContext directly if it is not nil
	if txContext = handler.getTxContext(channelID, txid); txContext == nil {
		errStr := fmt.Sprintf(fmtStr, args...)
		triggerNextStateMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(errStr), Txid: txid}
		return nil, triggerNextStateMsg
	}

	return txContext, nil
}

// Handles request to ledger to put state
func (handler *Handler) enterBusyState(e *fsm.Event, state string) {
	go func() {
		msg, _ := e.Args[0].(*pb.PtnChaincodeMessage)
		log.Debugf("[%s]state is %s", shorttxid(msg.Txid), state)
		// Check if this is the unique request from this chaincode txid
		uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
		if !uniqueReq {
			// Drop this request
			log.Debugf("Another request pending for this CC: %s, Txid: %s, ChannelID: %s. Cannot process.", handler.ChaincodeID.Name, msg.Txid, msg.ChannelId)
			return
		}

		var triggerNextStateMsg *pb.PtnChaincodeMessage
		var txContext *transactionContext
		txContext, triggerNextStateMsg = handler.getTxContextForMessage(msg.ChannelId, msg.Txid, msg.Type.String(), msg.Payload,
			"[%s]No ledger context for %s. Sending %s", shorttxid(msg.Txid), msg.Type.String(), pb.PtnChaincodeMessage_ERROR)

		defer func() {
			handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
			if triggerNextStateMsg != nil {
				log.Debugf("[%s]enterBusyState trigger event %s",
					shorttxid(triggerNextStateMsg.Txid), triggerNextStateMsg.Type)
				handler.triggerNextState(triggerNextStateMsg, true)
			}
		}()

		if txContext == nil {
			return
		}

		errHandler := func(payload []byte, errFmt string, errArgs ...interface{}) {
			log.Errorf(errFmt, errArgs...)
			triggerNextStateMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
		}
		//glh
		chaincodeID := handler.getCCRootName()
		var err error
		var res []byte

		if msg.Type.String() == pb.PtnChaincodeMessage_PTN_PUT_STATE.String() {
			putState := &pb.PtnPutState{}
			unmarshalErr := proto.Unmarshal(msg.Payload, putState)
			if unmarshalErr != nil {
				errHandler([]byte(unmarshalErr.Error()), "[%s]Unable to decipher payload. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)
				return
			}
			//vState := new(valueState)
			//err = json.Unmarshal(putState.Value, vState)
			//fmt.Println("vState===", vState)
			//fmt.Println()
			//glh
			/*
				if isCollectionSet(putState.Collection) {
					err = txContext.txsimulator.SetPrivateData(chaincodeID, putState.Collection, putState.Key, putState.Value)
				} else {
					err = txContext.txsimulator.SetState(chaincodeID, putState.Key, putState.Value)
				}
			*/
			if isCollectionSet(putState.Collection) {
				//err = txContext.txsimulator.SetPrivateData(chaincodeID, putState.Collection, putState.Key, putState.Value)
			} else {
				err = txContext.txsimulator.SetState(putState.ContractId, chaincodeID, putState.Key, putState.Value)
			}
		} else if msg.Type.String() == pb.PtnChaincodeMessage_PTN_DEL_STATE.String() {
			// Invoke ledger to delete state
			delState := &pb.PtnDelState{}
			unmarshalErr := proto.Unmarshal(msg.Payload, delState)
			if unmarshalErr != nil {
				errHandler([]byte(unmarshalErr.Error()), "[%s]Unable to decipher payload. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)
				return
			}

			//glh
			/*
				if isCollectionSet(delState.Collection) {
					err = txContext.txsimulator.DeletePrivateData(chaincodeID, delState.Collection, delState.Key)
				} else {
					err = txContext.txsimulator.DeleteState(chaincodeID, delState.Key)
				}
			*/
			if isCollectionSet(delState.Collection) {
				//err = txContext.txsimulator.DeletePrivateData(chaincodeID, delState.Collection, delState.Key)
			} else {
				err = txContext.txsimulator.DeleteState(delState.ContractId, chaincodeID, delState.Key)
			}
		} else if msg.Type.String() == pb.PtnChaincodeMessage_INVOKE_CHAINCODE.String() {
			log.Debugf("[%s] C-call-C", shorttxid(msg.Txid))
			chaincodeSpec := &pb.PtnChaincodeSpec{}
			unmarshalErr := proto.Unmarshal(msg.Payload, chaincodeSpec)
			if unmarshalErr != nil {
				errHandler([]byte(unmarshalErr.Error()), "[%s]Unable to decipher payload. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)
				return
			}

			// Get the chaincodeID to invoke. The chaincodeID to be called may
			// contain composite info like "chaincode-name:version/channel-name"
			// We are not using version now but default to the latest
			calledCcIns := getChaincodeInstance(chaincodeSpec.ChaincodeId.Name)
			chaincodeSpec.ChaincodeId.Name = calledCcIns.ChaincodeName
			if calledCcIns.ChainID == "" {
				// use caller's channel as the called chaincode is in the same channel
				calledCcIns.ChainID = txContext.chainID
			}
			log.Debugf("[%s] C-call-C %s on channel %s",
				shorttxid(msg.Txid), calledCcIns.ChaincodeName, calledCcIns.ChainID)

			//glh
			/*
				err := handler.checkACL(txContext.signedProp, txContext.proposal, calledCcIns)
				if err != nil {
					errHandler([]byte(err.Error()), "[%s] C-call-C %s on channel %s failed check ACL [%v]: [%s]", shorttxid(msg.Txid), calledCcIns.ChaincodeName, calledCcIns.ChainID, txContext.signedProp, err)
					return
				}
			*/
			// Set up a new context for the called chaincode if on a different channel
			// We grab the called channel's ledger simulator to hold the new state
			ctxt := context.Background()
			//glh
			/*
				txsim := txContext.txsimulator
				historyQueryExecutor := txContext.historyQueryExecutor
				if calledCcIns.ChainID != txContext.chainID {
					lgr := peer.GetLedger(calledCcIns.ChainID)
					if lgr == nil {
						payload := "Failed to find ledger for called channel " + calledCcIns.ChainID
						triggerNextStateMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR,
							Payload: []byte(payload), Txid: msg.Txid, ChannelId: msg.ChannelId}
						return
					}
					txsim2, err2 := lgr.NewTxSimulator(msg.Txid)
					if err2 != nil {
						triggerNextStateMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR,
							Payload: []byte(err2.Error()), Txid: msg.Txid, ChannelId: msg.ChannelId}
						return
					}
					defer txsim2.Done()
					txsim = txsim2
				}

				ctxt = context.WithValue(ctxt, TXSimulatorKey, txsim)
				ctxt = context.WithValue(ctxt, HistoryQueryExecutorKey, historyQueryExecutor)
			*/
			log.Debugf("[%s] getting chaincode data for %s on channel %s",
				shorttxid(msg.Txid), calledCcIns.ChaincodeName, calledCcIns.ChainID)

			//is the chaincode a system chaincode ?
			isscc := sysccprovider.GetSystemChaincodeProvider().IsSysCC(calledCcIns.ChaincodeName)

			var version string
			if !isscc {
				//if its a user chaincode, get the details
				//glh
				/*
					cd, err := GetChaincodeDefinition(ctxt, msg.Txid, txContext.signedProp, txContext.proposal, calledCcIns.ChainID, calledCcIns.ChaincodeName)
					if err != nil {
						errHandler([]byte(err.Error()), "[%s]Failed to get chaincode data (%s) for invoked chaincode. Sending %s", shorttxid(msg.Txid), err, pb.PtnChaincodeMessage_ERROR)
						return
					}


					version = cd.CCVersion()

					err = ccprovider.CheckInstantiationPolicy(calledCcIns.ChaincodeName, version, cd.(*ccprovider.ChaincodeData))
					if err != nil {
						errHandler([]byte(err.Error()), "[%s]CheckInstantiationPolicy, error %s. Sending %s", shorttxid(msg.Txid), err, pb.PtnChaincodeMessage_ERROR)
						return
					}
				*/
			} else {
				//this is a system cc, just call it directly
				version = util.GetSysCCVersion()
			}

			cccid := ccprovider.NewCCContext(msg.ContractId, calledCcIns.ChainID, calledCcIns.ChaincodeName, version, msg.Txid, false, txContext.signedProp, txContext.proposal)

			// Launch the new chaincode if not already running
			log.Debugf("[%s] launching chaincode %s on channel %s",
				shorttxid(msg.Txid), calledCcIns.ChaincodeName, calledCcIns.ChainID)
			cciSpec := &pb.PtnChaincodeInvocationSpec{ChaincodeSpec: chaincodeSpec}
			_, chaincodeInput, launchErr := handler.chaincodeSupport.Launch(ctxt, cccid, cciSpec)
			if launchErr != nil {
				errHandler([]byte(launchErr.Error()), "[%s]Failed to launch invoked chaincode. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)
				return
			}

			// TODO: Need to handle timeout correctly
			//timeout := time.Duration(30000) * time.Millisecond
			timeout := cfg.GetConfig().ContractDeploytimeout
			ccMsg, _ := createCCMessage(msg.ContractId, pb.PtnChaincodeMessage_TRANSACTION, calledCcIns.ChainID, msg.Txid, chaincodeInput)

			// Execute the chaincode... this CANNOT be an init at least for now
			response, execErr := handler.chaincodeSupport.Execute(ctxt, cccid, ccMsg, timeout)
			//log.Infof("----------------2-------------------------------%s\n\n\n\n\n", response)
			//log.Infof("-----------------2------------------------------%s\n\n\n\n\n", string(response.Payload))
			//payload is marshaled and send to the calling chaincode's shim which unmarshals and
			//sends it to chaincode
			res = nil
			if execErr != nil {
				err = execErr
			} else {
				res, err = proto.Marshal(response)
			}
		}

		if err != nil {
			errHandler([]byte(err.Error()), "[%s]Failed to handle %s. Sending %s", shorttxid(msg.Txid),
				msg.Type.String(), pb.PtnChaincodeMessage_ERROR)
			return
		}

		// Send response msg back to chaincode.
		log.Debugf("[%s]Completed %s. Sending %s", shorttxid(msg.Txid), msg.Type.String(), pb.PtnChaincodeMessage_RESPONSE)
		triggerNextStateMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: res, Txid: msg.Txid, ChannelId: msg.ChannelId}
	}()
}

// Handles request to ledger to OutChainCall
func (handler *Handler) enterOutChainCall(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	log.Debugf("[%s]Received %s, invoking OutChainCall from ledger", shorttxid(msg.Txid), pb.PtnChaincodeMessage_OUTCHAIN_CALL)

	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the afterGetState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode txid
		uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
		if !uniqueReq {
			// Drop this request
			log.Error("Another state request pending for this Txid. Cannot process.")
			return
		}

		var serialSendMsg *pb.PtnChaincodeMessage
		var txContext *transactionContext
		txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid,
			"[%s]No ledger context for OutChainCall. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)

		defer func() {
			handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
			if serialSendMsg != nil {
				log.Debugf("[%s]handle OutChainCall serial send %s",
					shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
				handler.serialSendAsync(serialSendMsg, nil)
			}
		}()

		if txContext == nil {
			return
		}

		key := string(msg.Payload)
		outChainCall := &pb.PtnOutChainCall{}
		unmarshalErr := proto.Unmarshal(msg.Payload, outChainCall)
		if unmarshalErr != nil {
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(unmarshalErr.Error()), Txid: msg.Txid, ChannelId: msg.ChannelId}
			return
		}
		chaincodeID := handler.getCCRootName()
		log.Debugf("[%s] OutChainCall %s, key %s, channel %s",
			shorttxid(msg.Txid), chaincodeID, outChainCall.OutChainName, txContext.chainID)

		var res []byte
		var err error
		result, err := outchain.ProcessOutChainCall(chaincodeID, outChainCall)
		if err == nil {
			res = []byte(result)
		}

		if err != nil {
			// Send error msg back to chaincode. OutChainCall will not trigger event
			payload := []byte(err.Error())
			log.Errorf("[%s]Failed to OutChainCall (%s). Sending %s",
				shorttxid(msg.Txid), err, pb.PtnChaincodeMessage_ERROR)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else if res == nil {
			//The state object being requested does not exist
			log.Debugf("[%s]No state associated with key: %s. Sending %s with an empty payload",
				shorttxid(msg.Txid), key, pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: res, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else {
			// Send response msg back to chaincode. GetState will not trigger event
			log.Debugf("[%s]OutChainCall. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: res, Txid: msg.Txid, ChannelId: msg.ChannelId}
		}
	}()
}

func (handler *Handler) enterSendJury(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	log.Debugf("[%s]Received %s, invoking get state from ledger", shorttxid(msg.Txid), pb.PtnChaincodeMessage_SEND_JURY)

	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the afterGetState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode txid
		uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
		if !uniqueReq {
			// Drop this request
			log.Error("Another state request pending for this Txid. Cannot process.")
			return
		}

		var serialSendMsg *pb.PtnChaincodeMessage
		var txContext *transactionContext
		txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid,
			"[%s]No ledger context for GetState. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)

		defer func() {
			handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
			if serialSendMsg != nil {
				log.Debugf("[%s]handleGetState serial send %s",
					shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
				handler.serialSendAsync(serialSendMsg, nil)
			}
		}()

		if txContext == nil {
			return
		}

		key := string(msg.Payload)
		sendJury := &pb.PtnSendJury{}
		unmarshalErr := proto.Unmarshal(msg.Payload, sendJury)
		if unmarshalErr != nil {
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(unmarshalErr.Error()), Txid: msg.Txid, ChannelId: msg.ChannelId}
			return
		}
		chaincodeID := handler.getCCRootName()
		log.Debugf("[%s] send jury for chaincode %s, msgType %d, channel %s",
			shorttxid(msg.Txid), chaincodeID, sendJury.MsgType, txContext.chainID)

		var res []byte
		var err error
		//if isCollectionSet(sendJury.Collection) {
		//	//glh
		//	//res, err = txContext.txsimulator.GetPrivateData(chaincodeID, getState.Collection, outChainAddr.Key)
		//} else {
		//	//glh
		//	//res, err = txContext.txsimulator.GetState(chaincodeID, getState.Key)
		//}
		//if txContext.txsimulator != nil {
		//	res, err = txContext.txsimulator.GetState(chaincodeID, getState.OutChainName)
		//}
		contractAddr, _ := common.StringToAddress(chaincodeID)
		result, err := handler.aJury.AdapterFunRequest(common.HexToHash(msg.Txid), contractAddr,
			sendJury.MsgType, sendJury.ConsultContent, sendJury.MyAnswer)
		if err == nil {
			res = result
		}

		if err != nil {
			// Send error msg back to chaincode. GetState will not trigger event
			payload := []byte(err.Error())
			log.Errorf("[%s]Failed to get chaincode state(%s). Sending %s",
				shorttxid(msg.Txid), err, pb.PtnChaincodeMessage_ERROR)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else if res == nil {
			//The state object being requested does not exist
			log.Debugf("[%s]No state associated with key: %s. Sending %s with an empty payload",
				shorttxid(msg.Txid), key, pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: res, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else {
			// Send response msg back to chaincode. GetState will not trigger event
			log.Debugf("[%s]Got state. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: res, Txid: msg.Txid, ChannelId: msg.ChannelId}
		}
	}()
}

func (handler *Handler) enterRecvJury(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	log.Debugf("[%s]Received %s, invoking get state from ledger", shorttxid(msg.Txid), pb.PtnChaincodeMessage_RECV_JURY)

	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the afterGetState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode txid
		uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
		if !uniqueReq {
			// Drop this request
			log.Error("Another state request pending for this Txid. Cannot process.")
			return
		}

		var serialSendMsg *pb.PtnChaincodeMessage
		var txContext *transactionContext
		txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid,
			"[%s]No ledger context for GetState. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)

		defer func() {
			handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
			if serialSendMsg != nil {
				log.Debugf("[%s]handleGetState serial send %s",
					shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
				handler.serialSendAsync(serialSendMsg, nil)
			}
		}()

		if txContext == nil {
			return
		}

		key := string(msg.Payload)
		recvJury := &pb.PtnRecvJury{}
		unmarshalErr := proto.Unmarshal(msg.Payload, recvJury)
		if unmarshalErr != nil {
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(unmarshalErr.Error()), Txid: msg.Txid, ChannelId: msg.ChannelId}
			return
		}
		chaincodeID := handler.getCCRootName()
		log.Debugf("[%s] getting state for chaincode %s, msgType %d, channel %s",
			shorttxid(msg.Txid), chaincodeID, recvJury.MsgType, txContext.chainID)

		var res []byte
		var err error
		//if isCollectionSet(recvJury.Collection) {
		//	//glh
		//	//res, err = txContext.txsimulator.GetPrivateData(chaincodeID, getState.Collection, outChainAddr.Key)
		//} else {
		//	//glh
		//	//res, err = txContext.txsimulator.GetState(chaincodeID, getState.Key)
		//}
		//if txContext.txsimulator != nil {
		//	res, err = txContext.txsimulator.GetState(chaincodeID, getState.OutChainName)
		//}
		contractAddr, _ := common.StringToAddress(chaincodeID)
		result, err := handler.aJury.AdapterFunResult(common.HexToHash(msg.Txid), contractAddr,
			recvJury.MsgType, recvJury.ConsultContent, time.Second*time.Duration(recvJury.Timeout))
		if err == nil {
			res = result
		}

		if err != nil {
			// Send error msg back to chaincode. GetState will not trigger event
			payload := []byte(err.Error())
			log.Errorf("[%s]Failed to get chaincode state(%s). Sending %s",
				shorttxid(msg.Txid), err, pb.PtnChaincodeMessage_ERROR)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else if res == nil {
			//The state object being requested does not exist
			log.Debugf("[%s]No state associated with key: %s. Sending %s with an empty payload",
				shorttxid(msg.Txid), key, pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: res, Txid: msg.Txid, ChannelId: msg.ChannelId}
		} else {
			// Send response msg back to chaincode. GetState will not trigger event
			log.Debugf("[%s]Got state. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_RESPONSE)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: res, Txid: msg.Txid, ChannelId: msg.ChannelId}
		}
	}()
}

func (handler *Handler) enterEstablishedState(e *fsm.Event) {
	log.Debugf("e %s", e.Dst)
	handler.notifyDuringStartup(true)
}

func (handler *Handler) enterReadyState(e *fsm.Event, state string) {
	// Now notify
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	log.Debugf("[%s]Entered state %s", shorttxid(msg.Txid), state)
	handler.notify(msg)
}

func (handler *Handler) enterEndState(e *fsm.Event, state string) {
	defer handler.deregister()
	// Now notify
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	log.Debugf("[%s]Entered state %s", shorttxid(msg.Txid), state)
	handler.notify(msg)
	e.Cancel(errors.New("entered end state"))
}

func (handler *Handler) setChaincodeProposal(signedProp *pb.PtnSignedProposal, prop *pb.PtnProposal, msg *pb.PtnChaincodeMessage) error {
	log.Debug("Setting chaincode proposal context...")
	if prop != nil {
		log.Debug("Proposal different from nil. Creating chaincode proposal context...")

		// Check that also signedProp is different from nil
		if signedProp == nil {
			return errors.New("failed getting proposal context. Signed proposal is nil")
		}

		msg.Proposal = signedProp
	} else {
		log.Debug("setChaincodeProposal error,prop is nil")
	}

	return nil
}

//move to ready
func (handler *Handler) ready(ctxt context.Context, chainID string, txid string, signedProp *pb.PtnSignedProposal, prop *pb.PtnProposal) (chan *pb.PtnChaincodeMessage, error) {
	txctx, funcErr := handler.createTxContext(ctxt, chainID, txid, signedProp, prop)
	if funcErr != nil {
		return nil, funcErr
	}

	log.Debug("sending READY")
	ccMsg := &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_READY, Txid: txid, ChannelId: chainID}

	//if security is disabled the context elements will just be nil
	if err := handler.setChaincodeProposal(signedProp, prop, ccMsg); err != nil {
		return nil, err
	}

	//send the ready synchronously as the
	//ready message is during launch and needs
	//to happen before any init/invokes can sneak in
	handler.triggerNextStateSync(ccMsg)

	return txctx.responseNotifier, nil
}

// handleMessage is the entrance method for Peer's handling of Chaincode messages.
func (handler *Handler) handleMessage(msg *pb.PtnChaincodeMessage) error {
	log.Debugf("[%s]Pallet peer side Handling ChaincodeMessage of type: %s in state %s", shorttxid(msg.Txid), msg.Type, handler.FSM.Current())

	if (msg.Type == pb.PtnChaincodeMessage_COMPLETED || msg.Type == pb.PtnChaincodeMessage_ERROR) && handler.FSM.Current() == "ready" {
		log.Debugf("[%s]HandleMessage- COMPLETED. Notify", msg.Txid)
		handler.notify(msg)
		return nil
	}
	if handler.FSM.Cannot(msg.Type.String()) {
		// Other errors
		return errors.Errorf("[%s]Chaincode handler validator FSM cannot handle message (%s) with payload size (%d) while in state: %s", msg.Txid, msg.Type.String(), len(msg.Payload), handler.FSM.Current())
	}
	eventErr := handler.FSM.Event(msg.Type.String(), msg)
	filteredErr := filterError(eventErr)
	if filteredErr != nil {
		log.Errorf("[%s]Failed to trigger FSM event %s: %s", msg.Txid, msg.Type.String(), filteredErr)
	}

	return filteredErr
}

func (handler *Handler) sendExecuteMessage(ctxt context.Context, chainID string, msg *pb.PtnChaincodeMessage, signedProp *pb.PtnSignedProposal, prop *pb.PtnProposal) (chan *pb.PtnChaincodeMessage, error) {
	log.Debugf("+++++++++++++++++ msg.Txid[%s]", msg.Txid)

	txctx, err := handler.createTxContext(ctxt, chainID, msg.Txid, signedProp, prop)
	if err != nil {
		log.Errorf("createTxContext [%s] error:%s", shorttxid(msg.Txid), err)
		return nil, err
	}

	log.Debugf("[%s]Inside sendExecuteMessage. Message %s", shorttxid(msg.Txid), msg.Type.String())

	//if security is disabled the context elements will just be nil
	if err = handler.setChaincodeProposal(signedProp, prop, msg); err != nil {
		log.Errorf("setChaincodeProposal [%s] error:%s", shorttxid(msg.Txid), err)
		return nil, err
	}

	log.Debugf("[%s]sendExecuteMsg trigger event %s", shorttxid(msg.Txid), msg.Type)
	handler.triggerNextState(msg, true)
	return txctx.responseNotifier, nil
}

func (handler *Handler) isRunning() bool {
	switch handler.FSM.Current() {
	case createdstate:
		fallthrough
	case establishedstate:
		fallthrough
	default:
		return true
	}
}

func (handler *Handler) enterGetCertByID(e *fsm.Event) {
	msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
	if !ok {
		e.Cancel(errors.New("received unexpected message type"))
		return
	}
	log.Debugf("[%s]Received %s, invoking get state from ledger", shorttxid(msg.Txid), pb.PtnChaincodeMessage_GET_CERT_STATE)
	// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
	// is completed before the next one is triggered. The previous state transition is deemed complete only when
	// the afterGetState function is exited. Interesting bug fix!!
	go func() {
		// Check if this is the unique state request from this chaincode txid
		uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
		if !uniqueReq {
			// Drop this request
			log.Error("Another state request pending for this Txid. Cannot process.")
			return
		}

		var serialSendMsg *pb.PtnChaincodeMessage
		var txContext *transactionContext
		txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid,
			"[%s]No ledger context for GetState. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)
		if txContext == nil {
			return
		}
		defer func() {
			handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
			log.Debugf("[%s]handleenterGetDepositConfig serial send %s",
				shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
			handler.serialSendAsync(serialSendMsg, nil)
		}()
		keyForSystemConfig := &pb.PtnKeyForSystemConfig{}
		unmarshalErr := proto.Unmarshal(msg.Payload, keyForSystemConfig)
		if unmarshalErr != nil {
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(unmarshalErr.Error()), Txid: msg.Txid, ChannelId: msg.ChannelId}
			return
		}
		chaincodeID := handler.getCCRootName()
		contractID := syscontract.DigitalIdentityContractAddress.Bytes()
		payloadBytes, err := txContext.txsimulator.GetState(contractID, chaincodeID, keyForSystemConfig.Key)
		log.Debugf("[%s] getting cert bytes for chaincode %s, channel %s", shorttxid(msg.Txid), chaincodeID, msg.ChannelId)
		if err != nil {
			log.Debugf("[%s]Got cert bytes. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)
			serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: []byte(err.Error()), Txid: msg.Txid, ChannelId: msg.ChannelId}
			return
		}
		log.Debugf("[%s]Got cert bytes. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_RESPONSE)
		serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: payloadBytes, Txid: msg.Txid, ChannelId: msg.ChannelId}
	}()
}

//glh
/*
func (handler *Handler) initializeQueryContext(txContext *transactionContext, queryID string,
   queryIterator commonledger.ResultsIterator) {
   handler.Lock()
   defer handler.Unlock()
   txContext.queryIteratorMap[queryID] = queryIterator
   txContext.pendingQueryResults[queryID] = &pendingQueryResult{batch: make([]*pb.QueryResultBytes, 0)}
}
*/
//func (handler *Handler) getQueryIterator(txContext *transactionContext, queryID string) commonledger.ResultsIterator {
// handler.Lock()
// defer handler.Unlock()
// return txContext.queryIteratorMap[queryID]
//}

//func (handler *Handler) cleanupQueryContext(txContext *transactionContext, queryID string) {
// handler.Lock()
// defer handler.Unlock()
// //glh
// //txContext.queryIteratorMap[queryID].Close()
// delete(txContext.queryIteratorMap, queryID)
// delete(txContext.pendingQueryResults, queryID)
//}

//glh
/*
// Check if the transactor is allow to call this chaincode on this channel
func (handler *Handler) checkACL(signedProp *pb.PtnSignedProposal, proposal *pb.PtnProposal, ccIns *sysccprovider.ChaincodeInstance) error {
   // ensure that we don't invoke a system chaincode
   // that is not invokable through a cc2cc invocation
   if sysccprovider.GetSystemChaincodeProvider().IsSysCCAndNotInvokableCC2CC(ccIns.ChaincodeName) {
      return errors.Errorf("system chaincode %s cannot be invoked with a cc2cc invocation", ccIns.ChaincodeName)
   }

   // if we are here, all we know is that the invoked chaincode is either
   // - a system chaincode that *is* invokable through a cc2cc
   //   (but we may still have to determine whether the invoker
   //   can perform this invocation)
   // - an application chaincode (and we still need to determine
   //   whether the invoker can invoke it)

   if sysccprovider.GetSystemChaincodeProvider().IsSysCC(ccIns.ChaincodeName) {
      // Allow this call
      return nil
   }

   // A Nil signedProp will be rejected for non-system chaincodes
   if signedProp == nil {
      return errors.Errorf("signed proposal must not be nil from caller [%s]", ccIns.String())
   }

   return aclmgmt.GetACLProvider().CheckACL(resources.CC2CC, ccIns.ChainID, signedProp)
}
*/

// afterGetStateByRange handles a GET_STATE_BY_RANGE request from the chaincode.
//func (handler *Handler) afterGetStateByRange(e *fsm.Event, state string) {
// msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
// if !ok {
//    e.Cancel(errors.New("received unexpected message type"))
//    return
// }
// log.Debugf("Received %s, invoking get state from ledger", pb.PtnChaincodeMessage_PTN_GET_STATE_BY_RANGE)
//
// // Query ledger for state
// handler.handleGetStateByRange(msg)
// log.Debug("Exiting GET_STATE_BY_RANGE")
//}

// Handles query to ledger to rage query state
//func (handler *Handler) handleGetStateByRange(msg *pb.PtnChaincodeMessage) {
//glh
// The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
// is completed before the next one is triggered. The previous state transition is deemed complete only when
// the afterGetStateByRange function is exited. Interesting bug fix!!
/*
   go func() {
      // Check if this is the unique state request from this chaincode txid
      uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
      if !uniqueReq {
         // Drop this request
         log.Error("Another state request pending for this Txid. Cannot process.")
         return
      }

      var serialSendMsg *pb.PtnChaincodeMessage

      defer func() {
         handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
         log.Debugf("[%s]handleGetStateByRange serial send %s", shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
         handler.serialSendAsync(serialSendMsg, nil)
      }()

      getStateByRange := &pb.PtnGetStateByRange{}
      unmarshalErr := proto.Unmarshal(msg.Payload, getStateByRange)
      if unmarshalErr != nil {
         payload := []byte(unmarshalErr.Error())
         log.Errorf("Failed to unmarshall range query request. Sending %s", pb.PtnChaincodeMessage_ERROR)
         serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
         return
      }

      iterID := util.GenerateUUID()

      var txContext *transactionContext

      txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid, "[%s]No ledger context for GetStateByRange. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)
      if txContext == nil {
         return
      }
      chaincodeID := handler.getCCRootName()

      errHandler := func(err error, iter commonledger.ResultsIterator, errFmt string, errArgs ...interface{}) {
         if iter != nil {
            handler.cleanupQueryContext(txContext, iterID)
         }
         payload := []byte(err.Error())
         log.Errorf(errFmt, errArgs...)
         serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
      }
      var rangeIter commonledger.ResultsIterator
      var err error

      if isCollectionSet(getStateByRange.Collection) {
         rangeIter, err = txContext.txsimulator.GetPrivateDataRangeScanIterator(chaincodeID, getStateByRange.Collection, getStateByRange.StartKey, getStateByRange.EndKey)
      } else {
         rangeIter, err = txContext.txsimulator.GetStateRangeScanIterator(chaincodeID, getStateByRange.StartKey, getStateByRange.EndKey)
      }
      if err != nil {
         errHandler(err, nil, "Failed to get ledger scan iterator. Sending %s", pb.PtnChaincodeMessage_ERROR)
         return
      }

      handler.initializeQueryContext(txContext, iterID, rangeIter)

      var payload *pb.QueryResponse
      payload, err = getQueryResponse(handler, txContext, rangeIter, iterID)
      if err != nil {
         errHandler(err, rangeIter, "Failed to get query result. Sending %s", pb.PtnChaincodeMessage_ERROR)
         return
      }

      var payloadBytes []byte
      payloadBytes, err = proto.Marshal(payload)
      if err != nil {
         errHandler(err, rangeIter, "Failed to marshal response. Sending %s", pb.PtnChaincodeMessage_ERROR)
         return
      }
      log.Debugf("Got keys and values. Sending %s", pb.PtnChaincodeMessage_RESPONSE)
      serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: payloadBytes, Txid: msg.Txid, ChannelId: msg.ChannelId}

   }()
*/
//}
/*
const maxResultLimit = 100
//getQueryResponse takes an iterator and fetch state to construct QueryResponse
func getQueryResponse(handler *Handler, txContext *transactionContext, iter commonledger.ResultsIterator,
   iterID string) (*pb.QueryResponse, error) {
   pendingQueryResults := txContext.pendingQueryResults[iterID]
   for {
      queryResult, err := iter.Next()
      switch {
      case err != nil:
         log.Errorf("Failed to get query result from iterator")
         handler.cleanupQueryContext(txContext, iterID)
         return nil, err
      case queryResult == nil:
         // nil response from iterator indicates end of query results
         batch := pendingQueryResults.cut()
         handler.cleanupQueryContext(txContext, iterID)
         return &pb.QueryResponse{Results: batch, HasMore: false, Id: iterID}, nil
      case pendingQueryResults.count == maxResultLimit:
         // max number of results queued up, cut batch, then add current result to pending batch
         batch := pendingQueryResults.cut()
         if err := pendingQueryResults.add(queryResult); err != nil {
            handler.cleanupQueryContext(txContext, iterID)
            return nil, err
         }
         return &pb.QueryResponse{Results: batch, HasMore: true, Id: iterID}, nil
      default:
         if err := pendingQueryResults.add(queryResult); err != nil {
            handler.cleanupQueryContext(txContext, iterID)
            return nil, err
         }
      }
   }
}
*/

// afterQueryStateNext handles a QUERY_STATE_NEXT request from the chaincode.
//func (handler *Handler) afterQueryStateNext(e *fsm.Event, state string) {
// msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
// if !ok {
//    e.Cancel(errors.New("received unexpected message type"))
//    return
// }
// log.Debugf("Received %s, invoking query state next from ledger", pb.PtnChaincodeMessage_QUERY_STATE_NEXT)
//
// // Query ledger for state
// handler.handleQueryStateNext(msg)
// log.Debug("Exiting QUERY_STATE_NEXT")
//}
/*
// Handles query to ledger for query state next
func (handler *Handler) handleQueryStateNext(msg *pb.PtnChaincodeMessage) {
   // The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
   // is completed before the next one is triggered. The previous state transition is deemed complete only when
   // the afterGetStateByRange function is exited. Interesting bug fix!!
   go func() {
      // Check if this is the unique state request from this chaincode txid
      uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
      if !uniqueReq {
         // Drop this request
         log.Debug("Another state request pending for this Txid. Cannot process.")
         return
      }

      var serialSendMsg *pb.PtnChaincodeMessage

      defer func() {
         handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
         log.Debugf("[%s]handleQueryStateNext serial send %s", shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
         handler.serialSendAsync(serialSendMsg, nil)
      }()

      var txContext *transactionContext
      var queryStateNext *pb.QueryStateNext

      errHandler := func(payload []byte, iter commonledger.ResultsIterator, errFmt string, errArgs ...interface{}) {
         if iter != nil {
            handler.cleanupQueryContext(txContext, queryStateNext.Id)
         }
         log.Errorf(errFmt, errArgs...)
         serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
      }

      queryStateNext = &pb.QueryStateNext{}

      unmarshalErr := proto.Unmarshal(msg.Payload, queryStateNext)
      if unmarshalErr != nil {
         errHandler([]byte(unmarshalErr.Error()), nil, "Failed to unmarshall state next query request. Sending %s", pb.PtnChaincodeMessage_ERROR)
         return
      }

      txContext = handler.getTxContext(msg.ChannelId, msg.Txid)
      if txContext == nil {
         errHandler([]byte("transaction context not found (timed out ?)"), nil, "[%s]Failed to get transaction context. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)
         return
      }

      queryIter := handler.getQueryIterator(txContext, queryStateNext.Id)

      if queryIter == nil {
         errHandler([]byte("query iterator not found"), nil, "query iterator not found. Sending %s", pb.PtnChaincodeMessage_ERROR)
         return
      }

      payload, err := getQueryResponse(handler, txContext, queryIter, queryStateNext.Id)
      if err != nil {
         errHandler([]byte(err.Error()), queryIter, "Failed to get query result. Sending %s", pb.PtnChaincodeMessage_ERROR)
         return
      }
      payloadBytes, err := proto.Marshal(payload)
      if err != nil {
         errHandler([]byte(err.Error()), queryIter, "Failed to marshal response. Sending %s", pb.PtnChaincodeMessage_ERROR)
         return
      }
      log.Debugf("Got keys and values. Sending %s", pb.PtnChaincodeMessage_RESPONSE)
      serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: payloadBytes, Txid: msg.Txid, ChannelId: msg.ChannelId}

   }()
}
*/
// afterQueryStateClose handles a QUERY_STATE_CLOSE request from the chaincode.
//func (handler *Handler) afterQueryStateClose(e *fsm.Event, state string) {
// msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
// if !ok {
//    e.Cancel(errors.New("received unexpected message type"))
//    return
// }
// log.Debugf("Received %s, invoking query state close from ledger", pb.PtnChaincodeMessage_QUERY_STATE_CLOSE)
//
// // Query ledger for state
// handler.handleQueryStateClose(msg)
// log.Debug("Exiting QUERY_STATE_CLOSE")
//}
/*
// Handles the closing of a state iterator
func (handler *Handler) handleQueryStateClose(msg *pb.PtnChaincodeMessage) {
   // The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
   // is completed before the next one is triggered. The previous state transition is deemed complete only when
   // the afterGetStateByRange function is exited. Interesting bug fix!!
   go func() {
      // Check if this is the unique state request from this chaincode txid
      uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
      if !uniqueReq {
         // Drop this request
         log.Error("Another state request pending for this Txid. Cannot process.")
         return
      }

      var serialSendMsg *pb.PtnChaincodeMessage

      defer func() {
         handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
         log.Debugf("[%s]handleQueryStateClose serial send %s", shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
         handler.serialSendAsync(serialSendMsg, nil)
      }()

      errHandler := func(payload []byte, errFmt string, errArgs ...interface{}) {
         log.Errorf(errFmt, errArgs...)
         serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
      }

      queryStateClose := &pb.QueryStateClose{}
      unmarshalErr := proto.Unmarshal(msg.Payload, queryStateClose)
      if unmarshalErr != nil {
         errHandler([]byte(unmarshalErr.Error()), "Failed to unmarshall state query close request. Sending %s", pb.PtnChaincodeMessage_ERROR)
         return
      }

      txContext := handler.getTxContext(msg.ChannelId, msg.Txid)
      if txContext == nil {
         errHandler([]byte("transaction context not found (timed out ?)"), "[%s]Failed to get transaction context. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)
         return
      }
      //glh
      /*
         iter := handler.getQueryIterator(txContext, queryStateClose.Id)
         if iter != nil {
            handler.cleanupQueryContext(txContext, queryStateClose.Id)
         }
*/
//    payload := &pb.QueryResponse{HasMore: false, Id: queryStateClose.Id}
//    payloadBytes, err := proto.Marshal(payload)
//    if err != nil {
//       errHandler([]byte(err.Error()), "Failed marshall resopnse. Sending %s", pb.PtnChaincodeMessage_ERROR)
//       return
//    }
//
//    log.Debugf("Closed. Sending %s", pb.PtnChaincodeMessage_RESPONSE)
//    serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: payloadBytes, Txid: msg.Txid, ChannelId: msg.ChannelId}
//
// }()
//}
//*/
// afterGetQueryResult handles a GET_QUERY_RESULT request from the chaincode.
//func (handler *Handler) afterGetQueryResult(e *fsm.Event, state string) {
//glh
/*
   msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
   if !ok {
      e.Cancel(errors.New("received unexpected message type"))
      return
   }
   log.Debugf("Received %s, invoking get state from ledger", pb.PtnChaincodeMessage_GET_QUERY_RESULT)

   // Query ledger for state
   handler.handleGetQueryResult(msg)
   log.Debug("Exiting GET_QUERY_RESULT")
*/
//}

//glh
// Handles query to ledger to execute query state
/*
func (handler *Handler) handleGetQueryResult(msg *pb.PtnChaincodeMessage) {
   // The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
   // is completed before the next one is triggered. The previous state transition is deemed complete only when
   // the afterQueryState function is exited. Interesting bug fix!!
   go func() {
      // Check if this is the unique state request from this chaincode txid
      uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
      if !uniqueReq {
         // Drop this request
         log.Error("Another state request pending for this Txid. Cannot process.")
         return
      }

      var serialSendMsg *pb.PtnChaincodeMessage

      defer func() {
         handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
         log.Debugf("[%s]handleGetQueryResult serial send %s", shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
         handler.serialSendAsync(serialSendMsg, nil)
      }()

      var txContext *transactionContext
      var iterID string

      errHandler := func(payload []byte, iter commonledger.ResultsIterator, errFmt string, errArgs ...interface{}) {
         if iter != nil {
            handler.cleanupQueryContext(txContext, iterID)
         }
         log.Errorf(errFmt, errArgs...)
         serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
      }

      getQueryResult := &pb.GetQueryResult{}
      unmarshalErr := proto.Unmarshal(msg.Payload, getQueryResult)
      if unmarshalErr != nil {
         errHandler([]byte(unmarshalErr.Error()), nil, "Failed to unmarshall query request. Sending %s", pb.PtnChaincodeMessage_ERROR)
         return
      }

      iterID = util.GenerateUUID()

      txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid, "[%s]No ledger context for GetQueryResult. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)
      if txContext == nil {
         return
      }

      chaincodeID := handler.getCCRootName()

      var err error
      var executeIter commonledger.ResultsIterator
      if isCollectionSet(getQueryResult.Collection) {
         executeIter, err = txContext.txsimulator.ExecuteQueryOnPrivateData(chaincodeID, getQueryResult.Collection, getQueryResult.Query)
      } else {
         executeIter, err = txContext.txsimulator.ExecuteQuery(chaincodeID, getQueryResult.Query)
      }

      if err != nil {
         errHandler([]byte(err.Error()), nil, "Failed to get ledger query iterator. Sending %s", pb.PtnChaincodeMessage_ERROR)
         return
      }

      handler.initializeQueryContext(txContext, iterID, executeIter)

      var payload *pb.QueryResponse
      payload, err = getQueryResponse(handler, txContext, executeIter, iterID)
      if err != nil {
         errHandler([]byte(err.Error()), executeIter, "Failed to get query result. Sending %s", pb.PtnChaincodeMessage_ERROR)
         return
      }

      var payloadBytes []byte
      payloadBytes, err = proto.Marshal(payload)
      if err != nil {
         errHandler([]byte(err.Error()), executeIter, "Failed marshall response. Sending %s", pb.PtnChaincodeMessage_ERROR)
         return
      }

      log.Debugf("Got keys and values. Sending %s", pb.PtnChaincodeMessage_RESPONSE)
      serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: payloadBytes, Txid: msg.Txid, ChannelId: msg.ChannelId}

   }()
}
*/
// afterGetHistoryForKey handles a GET_HISTORY_FOR_KEY request from the chaincode.
//func (handler *Handler) afterGetHistoryForKey(e *fsm.Event, state string) {
//glh
/*
   msg, ok := e.Args[0].(*pb.PtnChaincodeMessage)
   if !ok {
      e.Cancel(errors.New("received unexpected message type"))
      return
   }
   log.Debugf("Received %s, invoking get state from ledger", pb.PtnChaincodeMessage_GET_HISTORY_FOR_KEY)

   // Query ledger history db

   //handler.handleGetHistoryForKey(msg)
   log.Debug("Exiting GET_HISTORY_FOR_KEY")
*/
//}

//glh
// Handles query to ledger history db
/*
func (handler *Handler) handleGetHistoryForKey(msg *pb.PtnChaincodeMessage) {
   // The defer followed by triggering a go routine dance is needed to ensure that the previous state transition
   // is completed before the next one is triggered. The previous state transition is deemed complete only when
   // the afterQueryState function is exited. Interesting bug fix!!
   go func() {
      // Check if this is the unique state request from this chaincode txid
      uniqueReq := handler.createTXIDEntry(msg.ChannelId, msg.Txid)
      if !uniqueReq {
         // Drop this request
         log.Error("Another state request pending for this Txid. Cannot process.")
         return
      }

      var serialSendMsg *pb.PtnChaincodeMessage

      defer func() {
         handler.deleteTXIDEntry(msg.ChannelId, msg.Txid)
         log.Debugf("[%s]handleGetHistoryForKey serial send %s", shorttxid(serialSendMsg.Txid), serialSendMsg.Type)
         handler.serialSendAsync(serialSendMsg, nil)
      }()

      var iterID string
      var txContext *transactionContext

      errHandler := func(payload []byte, iter commonledger.ResultsIterator, errFmt string, errArgs ...interface{}) {
         if iter != nil {
            handler.cleanupQueryContext(txContext, iterID)
         }
         log.Errorf(errFmt, errArgs...)
         serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_ERROR, Payload: payload, Txid: msg.Txid, ChannelId: msg.ChannelId}
      }

      getHistoryForKey := &pb.GetHistoryForKey{}
      unmarshalErr := proto.Unmarshal(msg.Payload, getHistoryForKey)
      if unmarshalErr != nil {
         errHandler([]byte(unmarshalErr.Error()), nil, "Failed to unmarshall query request. Sending %s", pb.PtnChaincodeMessage_ERROR)
         return
      }

      iterID = util.GenerateUUID()

      txContext, serialSendMsg = handler.isValidTxSim(msg.ChannelId, msg.Txid, "[%s]No ledger context for GetHistoryForKey. Sending %s", shorttxid(msg.Txid), pb.PtnChaincodeMessage_ERROR)
      if txContext == nil {
         return
      }
      chaincodeID := handler.getCCRootName()

      historyIter, err := txContext.historyQueryExecutor.GetHistoryForKey(chaincodeID, getHistoryForKey.Key)
      if err != nil {
         errHandler([]byte(err.Error()), nil, "Failed to get ledger history iterator. Sending %s", pb.PtnChaincodeMessage_ERROR)
         return
      }

      handler.initializeQueryContext(txContext, iterID, historyIter)

      var payload *pb.QueryResponse
      payload, err = getQueryResponse(handler, txContext, historyIter, iterID)

      if err != nil {
         errHandler([]byte(err.Error()), historyIter, "Failed to get query result. Sending %s", pb.PtnChaincodeMessage_ERROR)
         return
      }

      var payloadBytes []byte
      payloadBytes, err = proto.Marshal(payload)
      if err != nil {
         errHandler([]byte(err.Error()), historyIter, "Failed marshal response. Sending %s", pb.PtnChaincodeMessage_ERROR)
         return
      }

      log.Debugf("Got keys and values. Sending %s", pb.PtnChaincodeMessage_RESPONSE)
      serialSendMsg = &pb.PtnChaincodeMessage{Type: pb.PtnChaincodeMessage_RESPONSE, Payload: payloadBytes, Txid: msg.Txid, ChannelId: msg.ChannelId}

   }()
}
*/
