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
 * @author PalletOne core developers <dev@pallet.one>
 * @date 2018
 */

package modules

import (
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/palletone/go-palletone/common"
	"github.com/palletone/go-palletone/common/log"
)

type transactionTemp struct {
	Version    uint32
	TxMessages []messageTemp
	TxExtra    []byte
}
type messageTemp struct {
	App  MessageType
	Data []byte
}
type txExtra struct {
	AccountNonce uint64
	CertId       []byte // should be big.Int byte
	Illegal      bool   // not hash, 1:no valid, 0:ok
}

func (tx *Transaction) DecodeRLP(s *rlp.Stream) error {
	raw, err := s.Raw()
	if err != nil {
		return err
	}
	txTemp := &transactionTemp{}
	err = rlp.DecodeBytes(raw, txTemp)
	if err != nil {
		txTempV0 := &transactionTempV0{}
		err = rlp.DecodeBytes(raw, txTempV0)
		if err != nil {
			return err
		}
		return tempV02Tx(txTempV0, tx)
	}
	return temp2Tx(txTemp, tx)
}
func (tx *Transaction) EncodeRLP(w io.Writer) error {
	if tx.Version() == 0 {
		return encodeRlpTxV0(tx, w)
	}
	temp, err := tx2Temp(tx)
	if err != nil {
		return err
	}
	return rlp.Encode(w, temp)
}
func encodeRlpTxV0(tx *Transaction, w io.Writer) error {
	tempV0, err := tx2TempV0(tx)
	if err != nil {
		return err
	}
	return rlp.Encode(w, tempV0)
}

func tx2Temp(tx *Transaction) (*transactionTemp, error) {
	temp := transactionTemp{}
	txExtra := txExtra{}
	txExtra.AccountNonce = tx.Nonce()
	txExtra.Illegal = tx.Illegal()
	txExtra.CertId = tx.CertId()
	txExtraB, err := rlp.EncodeToBytes(txExtra)
	if err != nil {
		return nil, err
	}
	temp.TxExtra = txExtraB
	temp.Version = tx.Version()
	for _, m := range tx.Messages() {
		m1 := messageTemp{
			App: m.App,
		}
		d, err := rlp.EncodeToBytes(m.Payload)
		if err != nil {
			return nil, err
		}
		m1.Data = d

		temp.TxMessages = append(temp.TxMessages, m1)

	}
	return &temp, nil
}
func temp2Tx(temp *transactionTemp, tx *Transaction) error {
	d := transaction_sdw{}
	for _, m := range temp.TxMessages {
		m1 := new(Message)
		m1.App = m.App
		if m.App == APP_PAYMENT {
			pay := new(PaymentPayload)
			err := rlp.DecodeBytes(m.Data, pay)
			if err != nil {
				return err
			}
			m1.Payload = pay
		} else if m.App == APP_DATA {
			var text DataPayload
			err := rlp.DecodeBytes(m.Data, &text)
			if err != nil {
				return err
			}
			m1.Payload = &text
		} else if m.App == APP_CONTRACT_TPL_REQUEST {
			var payload ContractInstallRequestPayload
			err := rlp.DecodeBytes(m.Data, &payload)
			if err != nil {
				return err
			}
			m1.Payload = &payload
		} else if m.App == APP_CONTRACT_TPL {
			var payload ContractTplPayload
			err := rlp.DecodeBytes(m.Data, &payload)
			if err != nil {
				return err
			}
			m1.Payload = &payload
		} else if m.App == APP_CONTRACT_DEPLOY_REQUEST {
			var payload ContractDeployRequestPayload
			err := rlp.DecodeBytes(m.Data, &payload)
			if err != nil {
				return err
			}
			m1.Payload = &payload
		} else if m.App == APP_CONTRACT_DEPLOY {
			var payload ContractDeployPayload
			log.Debugf("ContractDeployPayload hex:%x", m.Data)
			err := rlp.DecodeBytes(m.Data, &payload)
			if err != nil {
				log.Debugf("data [%x] cannot decode to newest ContractDeployPayload, try decode to"+
					" ContractDeployPayloadV1", m.Data)
				temp := &ContractDeployPayloadV1{}
				err = rlp.DecodeBytes(m.Data, temp)
				if err != nil {
					return err
				}

				payload.TemplateId = temp.TemplateId
				payload.ContractId = temp.ContractId
				payload.Name = temp.Name
				payload.Args = temp.Args
				payload.EleNode.EleList = temp.EleList
				payload.ReadSet = temp.ReadSet
				payload.WriteSet = temp.WriteSet
				payload.ErrMsg = temp.ErrMsg
			}
			m1.Payload = &payload
		} else if m.App == APP_CONTRACT_INVOKE_REQUEST {
			var payload ContractInvokeRequestPayload
			err := rlp.DecodeBytes(m.Data, &payload)
			if err != nil {
				return err
			}
			m1.Payload = &payload
		} else if m.App == APP_CONTRACT_INVOKE {
			var payload ContractInvokePayload
			err := rlp.DecodeBytes(m.Data, &payload)
			if err != nil {
				return err
			}
			m1.Payload = &payload
		} else if m.App == APP_CONTRACT_STOP_REQUEST {
			var payload ContractStopRequestPayload
			err := rlp.DecodeBytes(m.Data, &payload)
			if err != nil {
				return err
			}
			m1.Payload = &payload
		} else if m.App == APP_CONTRACT_STOP {
			var payload ContractStopPayload
			err := rlp.DecodeBytes(m.Data, &payload)
			if err != nil {
				return err
			}
			m1.Payload = &payload
			//} else if m.App == APP_CONFIG {
			//	var conf ConfigPayload
			//	rlp.DecodeBytes(m.Data, &conf)
			//	m1.Payload = &conf
		} else if m.App == APP_SIGNATURE {
			var sigPayload SignaturePayload
			err := rlp.DecodeBytes(m.Data, &sigPayload)
			if err != nil {
				return err
			}
			m1.Payload = &sigPayload
		} else if m.App == APP_ACCOUNT_UPDATE {
			var accountUpdateOp AccountStateUpdatePayload
			err := rlp.DecodeBytes(m.Data, &accountUpdateOp)
			if err != nil {
				return err
			}
			m1.Payload = &accountUpdateOp
		} else {
			fmt.Println("Unknown message app type:", m.App)
		}
		d.TxMessages = append(d.TxMessages, m1)

	}
	d.Version = temp.Version
	txExtra := new(txExtra)
	err := rlp.DecodeBytes(temp.TxExtra, &txExtra)
	if err != nil {
		return err
	}
	d.AccountNonce = txExtra.AccountNonce
	d.Illegal = txExtra.Illegal
	d.CertId = txExtra.CertId
	// init tx
	tx.txdata = d
	return nil
}

//RLP编码有Bug，在struct引用了空指针后，Decode会报错。所以将Input扁平化

type inputTemp struct {
	SignatureScript []byte
	Extra           []byte
	TxHash          common.Hash // reference Utxo struct key field
	MessageIndex    uint32      // message index in transaction
	OutIndex        uint32
}

func (input *Input) DecodeRLP(s *rlp.Stream) error {
	raw, err := s.Raw()
	if err != nil {
		return err
	}
	temp := &inputTemp{TxHash: common.Hash{}}
	err = rlp.DecodeBytes(raw, temp)
	if err != nil {
		return err
	}

	input.SignatureScript = temp.SignatureScript
	input.Extra = temp.Extra
	if !common.EmptyHash(temp.TxHash) {
		input.PreviousOutPoint = &OutPoint{TxHash: temp.TxHash, MessageIndex: temp.MessageIndex, OutIndex: temp.OutIndex}
	}
	return nil
}
func (input *Input) EncodeRLP(w io.Writer) error {
	temp := inputTemp{SignatureScript: input.SignatureScript, Extra: input.Extra}
	if input.PreviousOutPoint != nil {
		temp.TxHash = input.PreviousOutPoint.TxHash
		temp.MessageIndex = input.PreviousOutPoint.MessageIndex
		temp.OutIndex = input.PreviousOutPoint.OutIndex
	}
	return rlp.Encode(w, temp)
}
