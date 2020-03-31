/*
 *
 *    This file is part of go-palletone.
 *    go-palletone is free software: you can redistribute it and/or modify
 *    it under the terms of the GNU General Public License as published by
 *    the Free Software Foundation, either version 3 of the License, or
 *    (at your option) any later version.
 *    go-palletone is distributed in the hope that it will be useful,
 *    but WITHOUT ANY WARRANTY; without even the implied warranty of
 *    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *    GNU General Public License for more details.
 *    You should have received a copy of the GNU General Public License
 *    along with go-palletone.  If not, see <http://www.gnu.org/licenses/>.
 * /
 *
 *  * @author PalletOne core developer <dev@pallet.one>
 *  * @date 2018-2019
 *
 */

package txspool

import (
	"time"

	"github.com/ethereum/go-ethereum/event"
	"github.com/palletone/go-palletone/common"
	"github.com/palletone/go-palletone/common/ptndb"
	"github.com/palletone/go-palletone/core"
	"github.com/palletone/go-palletone/dag/modules"
	"github.com/palletone/go-palletone/validator"
)

type IDag interface {
	GetDb() ptndb.Database
	CurrentUnit(token modules.AssetId) *modules.Unit
	GetUnitByHash(hash common.Hash) (*modules.Unit, error)
	//GetTxFromAddress(tx *modules.Transaction) ([]common.Address, error)
	GetTransactionOnly(hash common.Hash) (*modules.Transaction, error)
	IsTransactionExist(hash common.Hash) (bool, error)
	GetTxHashByReqId(reqid common.Hash) (common.Hash, error)
	GetHeaderByHash(common.Hash) (*modules.Header, error)
	GetUtxoEntry(outpoint *modules.OutPoint) (*modules.Utxo, error)
	SubscribeChainHeadEvent(ch chan<- modules.ChainHeadEvent) event.Subscription
	// getTxfee
	//GetTxFee(pay *modules.Transaction) (*modules.AmountAsset, error)
	GetStxoEntry(outpoint *modules.OutPoint) (*modules.Stxo, error)
	GetTxOutput(outpoint *modules.OutPoint) (*modules.Utxo, error)

	GetContractTpl(tplId []byte) (*modules.ContractTemplate, error)
	GetContractJury(contractId []byte) (*modules.ElectionNode, error)
	GetContractState(id []byte, field string) ([]byte, *modules.StateVersion, error)
	GetContractStatesByPrefix(id []byte, prefix string) (map[string]*modules.ContractStateValue, error)
	GetTxRequesterAddress(tx *modules.Transaction) (common.Address, error)
	IsContractDeveloper(addr common.Address) bool

	GetMediators() map[common.Address]bool
	GetChainParameters() *core.ChainParameters
	GetNewestUnit(token modules.AssetId) (common.Hash, *modules.ChainIndex, error)
	GetNewestUnitTimestamp(token modules.AssetId) (int64, error)
	GetScheduledMediator(slotNum uint32) common.Address
	GetSlotAtTime(when time.Time) uint32
	GetMediator(add common.Address) *core.Mediator

	GetBlacklistAddress() ([]common.Address, *modules.StateVersion, error)
	GetJurorByAddrHash(addrHash common.Hash) (*modules.JurorDeposit, error)
	GetJurorReward(jurorAdd common.Address) common.Address
	GetAddrUtxoAndReqMapping(addr common.Address, asset *modules.Asset) (map[modules.OutPoint]*modules.Utxo, map[common.Hash]common.Hash, error)
	CheckReadSetValid(contractId []byte, readSet []modules.ContractReadSet) bool
}

type IValidator interface {
	ValidateTx(tx *modules.Transaction, isFullTx bool) ([]*modules.Addition, validator.ValidationCode, error)
	SetUtxoQuery(query validator.IUtxoQuery)
}
