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

package rwset

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/palletone/go-palletone/common"
	"github.com/palletone/go-palletone/common/log"

	"github.com/palletone/go-palletone/dag/dagconfig"
	"github.com/palletone/go-palletone/dag/modules"
)

type RwSetTxSimulator struct {
	txId         string
	rwsetBuilder *RWSetBuilder
	dag          IDataQuery
	stateQuery   IStateQuery
	doneInvoked  bool
}

func NewBasedTxSimulator(txId string, idag IDataQuery, stateQ IStateQuery) *RwSetTxSimulator {
	return &RwSetTxSimulator{
		txId:         txId,
		rwsetBuilder: NewRWSetBuilder(),
		stateQuery:   stateQ,
		dag:          idag}
}

func (s *RwSetTxSimulator) GetGlobalProp() ([]byte, error) {
	gp := s.dag.GetGlobalProp()

	data, err := rlp.EncodeToBytes(gp)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// GetState implements method in interface `ledger.TxSimulator`
func (s *RwSetTxSimulator) GetState(contractid []byte, ns string, key string) ([]byte, error) {
	//testValue := []byte("abc")
	if err := s.CheckDone(); err != nil {
		return nil, err
	}
	val, ver, err := s.GetContractState(contractid, key)
	//TODO 这里证明数据库里面没有该账户信息，需要返回nil,nil
	if err != nil {
		log.Debugf("get value from db[%s] failed,key:%s", ns, key)
		return nil, nil
		//errstr := fmt.Sprintf("GetContractState [%s]-[%s] failed", ns, key)
		//		//return nil, errors.New(errstr)
	}
	if ver != nil && ver.TxIndex == 0 { //Devin Debug
		log.DebugDynamic(func() string {
			logStr := "call stack:"
			var query = s.stateQuery
			for {
				if ss, ok := query.(*RwSetTxSimulator); ok {
					query = ss.stateQuery
					logStr += ss.txId + ";"
				} else {
					logStr += reflect.TypeOf(query).String()
					break
				}
			}
			return logStr
		})
		log.Warn("tx index==0")
	}
	if s.rwsetBuilder != nil {
		s.rwsetBuilder.AddToReadSet(contractid, ns, key, ver)
	}
	log.Debugf("RW:GetState,ns[%s]--key[%s]---value[%s]---ver[%v]", ns, key, val, ver)

	//TODO change.
	//return testValue, nil
	return val, nil
}
func (s *RwSetTxSimulator) GetStatesByPrefix(contractid []byte, ns string, prefix string) ([]*modules.KeyValue, error) {
	if err := s.CheckDone(); err != nil {
		return nil, err
	}

	data, err := s.GetContractStatesByPrefix(contractid, prefix)

	if err != nil {
		log.Debugf("get value from db[%s] failed,prefix:%s,error:[%s]", ns, prefix, err.Error())
		return nil, nil
		//errstr := fmt.Sprintf("GetContractState [%s]-[%s] failed", ns, key)
		//		//return nil, errors.New(errstr)
	}
	result := []*modules.KeyValue{}
	//  保持 key 返回一致
	sliceKeys := mapKeyToSlice(data)
	for _, key := range sliceKeys {
		kv := &modules.KeyValue{Key: key, Value: data[key].Value}
		result = append(result, kv)
		if s.rwsetBuilder != nil {
			s.rwsetBuilder.AddToReadSet(contractid, ns, key, data[key].Version)
		}
	}

	//log.Debugf("RW:GetStatesByPrefix,ns[%s]--contractid[%x]---prefix[%s]", ns, contractid, prefix)

	return result, nil
}

// GetState implements method in interface `ledger.TxSimulator`
func (s *RwSetTxSimulator) GetTimestamp(ns string, rangeNumber uint32) ([]byte, error) {
	//testValue := []byte("abc")
	if err := s.CheckDone(); err != nil {
		return nil, err
	}
	gasToken := dagconfig.DagConfig.GetGasToken()
	_, index, _ := s.dag.GetNewestUnit(gasToken)
	timeIndex := index.Index / uint64(rangeNumber) * uint64(rangeNumber)
	timeHeader, err := s.dag.GetHeaderByNumber(&modules.ChainIndex{AssetID: index.AssetID, Index: timeIndex})
	if err != nil {
		return nil, errors.New("GetHeaderByNumber failed" + err.Error())
	}

	return []byte(fmt.Sprintf("%d", timeHeader.Timestamp())), nil
}
func (s *RwSetTxSimulator) SetState(contractId []byte, ns string, key string, value []byte) error {
	if err := s.CheckDone(); err != nil {
		return err
	}

	//todo ValidateKeyValue
	s.rwsetBuilder.AddToWriteSet(contractId, ns, key, value)

	return nil
}

// DeleteState implements method in interface `ledger.TxSimulator`
func (s *RwSetTxSimulator) DeleteState(contractId []byte, ns string, key string) error {
	return s.SetState(contractId, ns, key, nil)
	//TODO Devin
}

func (s *RwSetTxSimulator) GetRwData(ns string) ([]*KVRead, []*KVWrite, error) {
	rd := make(map[string]map[string]*KVRead)
	wt := make(map[string]map[string]*KVWrite)
	log.Debug("GetRwData", "ns info", ns)

	if s.rwsetBuilder != nil {
		s.rwsetBuilder.locker.RLock()
		if s.rwsetBuilder.pubRwBuilderMap != nil {
			if s.rwsetBuilder.pubRwBuilderMap[ns] != nil {
				pubRwBuilderMap, ok := s.rwsetBuilder.pubRwBuilderMap[ns]
				if ok {
					rd = pubRwBuilderMap.readMap
					wt = pubRwBuilderMap.writeMap
				} else {
					s.rwsetBuilder.locker.RUnlock()
					return nil, nil, errors.New("rw_data not found.")
				}
			}
		}
		s.rwsetBuilder.locker.RUnlock()
	}
	//sort keys and convert map to slice
	return convertReadMap2Slice(rd), convertWriteMap2Slice(wt), nil
}
func convertReadMap2Slice(rd map[string]map[string]*KVRead) []*KVRead {
	result := make([]*KVRead, 0)
	for _, kv := range rd {
		for _, v := range kv {
			result = append(result, v)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].key < result[j].key
	})

	return result
}
func convertWriteMap2Slice(rd map[string]map[string]*KVWrite) []*KVWrite {
	result := make([]*KVWrite, 0)
	for _, kv := range rd {
		for _, v := range kv {
			result = append(result, v)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].key < result[j].key
	})

	return result
}

//get all dag
func (s *RwSetTxSimulator) GetAllStates(contractid []byte, ns string) (map[string]*modules.ContractStateValue, error) {
	return s.GetContractStatesById(contractid)
}

func (s *RwSetTxSimulator) CheckDone() error {
	if s.doneInvoked {
		return errors.New("This instance should not be used after calling Done()")
	}
	return nil
}

//标记为完成，不能再进行PutState等写操作了
func (h *RwSetTxSimulator) Done() {
	if h.doneInvoked {
		return
	}
	//h.Close()
	h.doneInvoked = true
}

//关闭，回收资源，不能再进行读和写
func (s *RwSetTxSimulator) Close() {
	log.Debugf("Close RwSetTxSimulator[%s]", s.txId)
	item := new(RwSetTxSimulator)
	//s.chainIndex = item.chainIndex
	//s.txid = item.txid
	s.rwsetBuilder = item.rwsetBuilder
	//s.write_cache = item.write_cache
	s.dag = item.dag
}

func (s *RwSetTxSimulator) Rollback() error {
	if s.rwsetBuilder != nil {
		s.rwsetBuilder.pubRwBuilderMap = make(map[string]*nsPubRwBuilder)
	}
	log.Infof("Rollback tx simulator[%s]", s.txId)
	return nil
}

func (s *RwSetTxSimulator) GetTokenBalance(ns string, addr common.Address, asset *modules.Asset) (
	map[modules.Asset]uint64, error) {
	var utxos map[modules.OutPoint]*modules.Utxo
	if asset == nil {
		utxos, _ = s.dag.GetAddrUtxos(addr)
	} else {
		utxos, _ = s.dag.GetAddr1TokenUtxos(addr, asset)
	}
	return convertUtxo2Balance(utxos), nil
}

func (s *RwSetTxSimulator) GetStableTransactionByHash(ns string, hash common.Hash) (*modules.Transaction, error) {
	return s.dag.GetStableTransactionOnly(hash)
}

func (s *RwSetTxSimulator) GetStableUnit(ns string, hash common.Hash, unitNumber uint64) (*modules.Unit, error) {
	if !hash.IsZero() {
		return s.dag.GetStableUnit(hash)
	}
	gasToken := dagconfig.DagConfig.GetGasToken()
	number := &modules.ChainIndex{AssetID: gasToken, Index: unitNumber}
	return s.dag.GetStableUnitByNumber(number)
}
func convertUtxo2Balance(utxos map[modules.OutPoint]*modules.Utxo) map[modules.Asset]uint64 {
	result := map[modules.Asset]uint64{}
	for _, v := range utxos {
		if val, ok := result[*v.Asset]; ok {
			result[*v.Asset] = val + v.Amount
		} else {
			result[*v.Asset] = v.Amount
		}
	}
	return result
}

func (s *RwSetTxSimulator) PayOutToken(ns string, address string, token *modules.Asset, amount uint64,
	lockTime uint32) error {
	addr, err := common.StringToAddress(address)
	if err != nil {
		return err
	}
	s.rwsetBuilder.AddTokenPayOut(ns, addr, token, amount, lockTime)
	return nil
}

func (s *RwSetTxSimulator) GetPayOutData(ns string) ([]*modules.TokenPayOut, error) {
	return s.rwsetBuilder.GetTokenPayOut(ns), nil
}

func (s *RwSetTxSimulator) GetTokenDefineData(ns string) (*modules.TokenDefine, error) {
	return s.rwsetBuilder.GetTokenDefine(ns), nil
}

func (s *RwSetTxSimulator) GetTokenSupplyData(ns string) ([]*modules.TokenSupply, error) {
	return s.rwsetBuilder.GetTokenSupply(ns), nil
}

func (s *RwSetTxSimulator) DefineToken(ns string, tokenType int32, define []byte, creator string) error {
	createAddr, _ := common.StringToAddress(creator)
	s.rwsetBuilder.DefineToken(ns, tokenType, define, createAddr)
	return nil
}

func (s *RwSetTxSimulator) SupplyToken(ns string, assetId, uniqueId []byte, amt uint64, creator string) error {
	createAddr, _ := common.StringToAddress(creator)
	return s.rwsetBuilder.AddSupplyToken(ns, assetId, uniqueId, amt, createAddr)
}

func (s *RwSetTxSimulator) String() string {
	str := "rwSet_txSimulator: "
	for k, v := range s.rwsetBuilder.pubRwBuilderMap {
		str += "key:" + k
		for rk, rv := range v.readMap {
			//str += fmt.Sprintf("val__[key:%s],[value:%s]", rk, rv.String())
			log.Debug("RwSetTxSimulator) String", "key", rk, "val-", rv)
		}
	}
	return str
}

func (s *RwSetTxSimulator) GetContractStatesById(contractid []byte) (map[string]*modules.ContractStateValue, error) {
	//查询出所有Temp的KeyValue
	writes, _ := s.rwsetBuilder.GetWriteSets(contractid)
	deleted := make(map[string]bool)
	result := make(map[string]*modules.ContractStateValue)
	for _, write := range writes {
		if write.isDelete {
			deleted[write.key] = true
		} else {
			result[write.key] = &modules.ContractStateValue{Value: write.value}
		}
	}

	dbkv, err := s.stateQuery.GetContractStatesById(contractid)
	if err != nil {
		return nil, err
	}
	for k, v := range dbkv {
		if _, ok := result[k]; ok {
			continue
		}
		if _, ok := deleted[k]; ok {
			continue
		}
		result[k] = v
	}
	return result, nil
}

func (s *RwSetTxSimulator) GetContractState(contractid []byte, field string) ([]byte, *modules.StateVersion, error) {
	value, err := s.rwsetBuilder.GetWriteSet(contractid, field)
	if err != nil {
		return s.stateQuery.GetContractState(contractid, field)
	}
	log.Debugf("Get contract state key[%s] from rwset builder", field)
	return value, nil, nil
}

func (s *RwSetTxSimulator) GetContractStatesByPrefix(contractid []byte, prefix string) (map[string]*modules.ContractStateValue, error) {
	//查询出所有Temp的KeyValue
	writes, _ := s.rwsetBuilder.GetWriteSets(contractid)
	deleted := make(map[string]bool)
	result := make(map[string]*modules.ContractStateValue)
	for _, write := range writes {
		if strings.HasPrefix(write.key, prefix) {
			if write.isDelete {
				deleted[write.key] = true
			} else {
				result[write.key] = &modules.ContractStateValue{Value: write.value}
			}
		}
	}

	dbkv, err := s.stateQuery.GetContractStatesByPrefix(contractid, prefix)
	if err != nil {
		return nil, err
	}
	for k, v := range dbkv {
		if _, ok := result[k]; ok {
			continue
		}
		if _, ok := deleted[k]; ok {
			continue
		}
		result[k] = v
	}
	return result, nil
}

func mapKeyToSlice(m map[string]*modules.ContractStateValue) []string {
	sliceKeys := []string{}
	for key := range m {
		sliceKeys = append(sliceKeys, key)
	}
	sort.Strings(sliceKeys)
	return sliceKeys
}
