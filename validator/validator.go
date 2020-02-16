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

package validator

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/palletone/go-palletone/common"
	"github.com/palletone/go-palletone/common/log"
	"github.com/palletone/go-palletone/dag/dagconfig"
	"github.com/palletone/go-palletone/dag/dboperation"
	"github.com/palletone/go-palletone/dag/modules"
	"github.com/palletone/go-palletone/dag/palletcache"
	"github.com/palletone/go-palletone/dag/parameter"
	"github.com/palletone/go-palletone/dag/rwset"
	"github.com/palletone/go-palletone/tokenengine"
)

type ContractTxCheckFunc func(tx *modules.Transaction, rwM rwset.TxManager, dag dboperation.IContractDag) bool
type BuildTempContractDagFunc func(dag dboperation.IContractDag) dboperation.IContractDag
type Validate struct {
	utxoquery                IUtxoQuery
	statequery               IStateQuery
	dagquery                 IDagQuery
	propquery                IPropQuery
	contractDb               dboperation.IContractDag
	tokenEngine              tokenengine.ITokenEngine
	cache                    *ValidatorCache
	enableTxFeeCheck         bool
	enableContractSignCheck  bool
	enableDeveloperCheck     bool
	enableContractRwSetCheck bool
	light                    bool
	contractCheckFun         ContractTxCheckFunc //合约检查函数，通过Set方法注入
	//buildTempDagFunc         BuildTempContractDagFunc //为合约运行构造临时Db的方法，通过Set注入
}

func NewValidate(dagdb IDagQuery, utxoRep IUtxoQuery, statedb IStateQuery, propquery IPropQuery,
	contractDag dboperation.IContractDag,
	cache palletcache.ICache, light bool) Validator {
	//cache := freecache.NewCache(20 * 1024 * 1024)
	vcache := NewValidatorCache(cache)
	return &Validate{
		cache:                    vcache,
		dagquery:                 dagdb,
		utxoquery:                utxoRep,
		statequery:               statedb,
		propquery:                propquery,
		contractDb:               contractDag,
		tokenEngine:              tokenengine.Instance,
		enableTxFeeCheck:         true,
		enableContractSignCheck:  true,
		enableDeveloperCheck:     true,
		enableContractRwSetCheck: true,
		light:                    light,
	}
}

type newUtxoQuery struct {
	oldUtxoQuery IUtxoQuery
	unitUtxo     *sync.Map
}

func (q *newUtxoQuery) GetStxoEntry(outpoint *modules.OutPoint) (*modules.Stxo, error) {
	return q.oldUtxoQuery.GetStxoEntry(outpoint)
}
func (q *newUtxoQuery) GetUtxoEntry(outpoint *modules.OutPoint) (*modules.Utxo, error) {
	utxo, ok := q.unitUtxo.Load(*outpoint)
	if ok {
		return utxo.(*modules.Utxo), nil
	}
	return q.oldUtxoQuery.GetUtxoEntry(outpoint)
}
func (validate *Validate) setUtxoQuery(q IUtxoQuery) {
	validate.utxoquery = q
}

//逐条验证每一个Tx，并返回总手续费的分配情况，然后与Coinbase进行比较
func (validate *Validate) validateTransactions(rwM rwset.TxManager, txs modules.Transactions, unitTime int64, unitAuthor common.Address) ValidationCode {
	ads := make([]*modules.Addition, 0)

	oldUtxoQuery := validate.utxoquery

	unitUtxo := new(sync.Map)
	newUtxoQuery := &newUtxoQuery{oldUtxoQuery: oldUtxoQuery, unitUtxo: unitUtxo}
	validate.utxoquery = newUtxoQuery
	defer validate.setUtxoQuery(oldUtxoQuery)
	spendOutpointMap := make(map[*modules.OutPoint]common.Hash)
	var coinbase *modules.Transaction
	//构造TempDag用于存储Tx的结果
	if validate.contractDb != nil {
		validate.contractDb, _ = validate.contractDb.NewTemp()
	}
	//tempdb, err := ptndb.NewTempdb(validate.db)
	//if err != nil {
	//	log.Errorf("Init tempdb error:%s", err.Error())
	//}
	//tempDag := common2.NewUnitRepository4Db(tempdb,tokenengine.Instance)
	//if err != nil {
	//	log.Errorf("Init temp dag error:%s", err.Error())
	//}
	//validate.contractDb=tempDag
	for txIndex, tx := range txs {
		//先检查普通交易并计算手续费，最后检查Coinbase
		txHash := tx.Hash()
		if validate.checkTxIsExist(tx) {
			return TxValidationCode_DUPLICATE_TXID
		}
		if txIndex == 0 {
			coinbase = tx
			continue
			//每个单元的第一条交易比较特殊，是Coinbase交易，其包含增发和收集的手续费

		}
		txFeeAllocate, txCode, _ := validate.validateTxAndCache(rwM, tx, true)
		if txCode != TxValidationCode_VALID {
			log.Debug("ValidateTx", "txhash", txHash, "error validate code", txCode)
			return txCode
		}
		// 验证双花
		for _, outpoint := range tx.GetSpendOutpoints() {
			if spentTx, ok := spendOutpointMap[outpoint]; ok {
				log.Errorf("Utxo[%s] spent by tx[%s]", outpoint.String(), spentTx.String())
				return TxValidationCode_INVALID_DOUBLE_SPEND
			}
			if stxo, _ := validate.utxoquery.GetStxoEntry(outpoint); stxo != nil {
				log.Errorf("Utxo[%s] spent by tx[%s]", outpoint.String(), stxo.SpentByTxId.String())
				return TxValidationCode_INVALID_DOUBLE_SPEND
			}
			spendOutpointMap[outpoint] = txHash
		}

		for _, a := range txFeeAllocate {
			if a.Addr.IsZero() {
				a.Addr = unitAuthor
			}
			ads = append(ads, a)
		}

		for outPoint, utxo := range tx.GetNewUtxos() {
			log.Debugf("Add tx utxo for key:%s", outPoint.String())
			unitUtxo.Store(outPoint, utxo)
		}
		if validate.contractDb != nil {
			validate.contractDb.SaveTransaction(tx)
		}
		//tempDag.SaveTransaction(tx)
		//newUtxoQuery.unitUtxo = unitUtxo
		//validate.utxoquery = newUtxoQuery
	}
	//验证第一条交易
	if len(txs) > 0 {
		//附加上出块奖励
		a := &modules.Addition{
			Addr:   unitAuthor,
			Amount: parameter.CurrentSysParameters.GenerateUnitReward,
			Asset:  dagconfig.DagConfig.GetGasToken().ToAsset(),
		}
		ads = append(ads, a)
		out := arrangeAdditionFeeList(ads)
		log.DebugDynamic(func() string {
			data, _ := json.Marshal(out)
			return "Fee allocation:" + string(data)
		})
		//手续费应该与其他交易付出的手续费相等
		if unitTime > 1564675200 { //2019.8.2主网升级，有些之前的Coinbase可能验证不过。所以主网升级前的不验证了
			coinbaseValidateResult := validate.validateCoinbase(coinbase, out)
			if coinbaseValidateResult == TxValidationCode_VALID {
				log.Debugf("Validate coinbase[%s] pass", coinbase.Hash().String())
			} else {
				log.DebugDynamic(func() string {
					data, _ := json.Marshal(coinbase)
					return fmt.Sprintf("Coinbase[%s] invalid, content: %s", coinbase.Hash().String(), string(data))
				})
			}
			return coinbaseValidateResult
		}

	}
	return TxValidationCode_VALID
}

func arrangeAdditionFeeList(ads []*modules.Addition) []*modules.Addition {
	if len(ads) <= 0 {
		return nil
	}
	out := make(map[string]*modules.Addition)
	for _, a := range ads {
		key := a.Key()
		b, ok := out[key]
		if ok {
			b.Amount += a.Amount
		} else {
			out[key] = a
		}
	}
	if len(out) < 1 {
		return nil
	}
	result := make([]*modules.Addition, 0)
	for _, v := range out {
		result = append(result, v)
	}
	return result
}

/**
检查unit中所有交易的合法性，返回所有交易的交易费总和
check all transactions in one unit
return all transactions' fee
*/
//func (validate *Validate) ValidateTransactions(txs modules.Transactions) error {
//	code := validate.validateTransactions(txs)
//	return NewValidateError(code)
//}

func (validate *Validate) ValidateTx(tx *modules.Transaction, isFullTx bool) ([]*modules.Addition, ValidationCode, error) {
	txId := tx.Hash()
	has, add := validate.cache.HasTxValidateResult(txId)
	if has {
		return add, TxValidationCode_VALID, nil
	}
	validate.enableTxFeeCheck = true
	validate.enableContractSignCheck = true
	validate.enableDeveloperCheck = true
	validate.enableContractRwSetCheck = true
	code, addition := validate.validateTx(nil, tx, isFullTx)
	if code == TxValidationCode_VALID {
		validate.cache.AddTxValidateResult(txId, addition)
		return addition, code, nil
	}
	return addition, code, NewValidateError(code)
}
func (validate *Validate) validateTxAndCache(rwM rwset.TxManager, tx *modules.Transaction, isFullTx bool) ([]*modules.Addition, ValidationCode, error) {
	txId := tx.Hash()
	has, add := validate.cache.HasTxValidateResult(txId)
	if has {
		return add, TxValidationCode_VALID, nil
	}
	code, addition := validate.validateTx(rwM, tx, isFullTx)
	if code == TxValidationCode_VALID {
		validate.cache.AddTxValidateResult(txId, addition)
		return addition, code, nil
	}
	return nil, code, NewValidateError(code)
}

// todo
// 验证群签名接口，需要验证群签的正确性和有效性
func (validate *Validate) ValidateUnitGroupSign(h *modules.Header) error {
	return nil
}

//验证一个DataPayment
func (validate *Validate) validateDataPayload(payload *modules.DataPayload) ValidationCode {
	//验证 maindata是否存在
	//验证 maindata extradata大小 不可过大
	//len(payload.MainData) >= MAX_DATA_PAYLOAD_MAIN_DATA_SIZE
	if len(payload.MainData) == 0 {
		return TxValidationCode_INVALID_DATAPAYLOAD
	}
	//TODO 验证maindata其它属性
	return TxValidationCode_VALID
}
func (validate *Validate) CheckTxIsExist(tx *modules.Transaction) bool {
	return validate.checkTxIsExist(tx)
}
func (validate *Validate) checkTxIsExist(tx *modules.Transaction) bool {
	if len(tx.TxMessages()) > 2 {
		txHash := tx.Hash()
		if validate.dagquery == nil {
			log.Warnf("Validate DagQuery doesn't set, cannot check tx[%s] is exist or not", txHash.String())
			return false
		}
		if has, _ := validate.dagquery.IsTransactionExist(txHash); has {
			log.Debug("checkTxIsExist transactions exist in dag", "txHash", txHash.String())
			return true
		}
	}
	return false
}

//func (validate *Validate) ContractTxCheck(rwM rwset.TxManager, tx *modules.Transaction) bool {
//	if validate.contractCheckFun != nil {
//		return validate.contractCheckFun(rwM,dag, tx)
//	}
//	return true
//}
func (validate *Validate) SetContractTxCheckFun(checkFun ContractTxCheckFunc) {
	validate.contractCheckFun = checkFun
	log.Debug("SetContractTxCheckFun ok")
}

//func (v *Validate) SetBuildTempContractDagFunc(buildFunc BuildTempContractDagFunc) {
//	v.buildTempDagFunc = buildFunc
//	log.Debug("SetBuildTempContractDagFunc ok")
//}
