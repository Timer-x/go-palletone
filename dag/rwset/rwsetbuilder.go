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
	"bytes"
	"github.com/palletone/go-palletone/common"
	"github.com/palletone/go-palletone/dag/errors"
	"github.com/palletone/go-palletone/dag/modules"
)

type RWSetBuilder struct {
	pubRwBuilderMap map[string]*nsPubRwBuilder
}

type nsPubRwBuilder struct {
	namespace   string
	readMap     map[string]*KVRead
	writeMap    map[string]*KVWrite
	tokenPayOut []*modules.TokenPayOut
	tokenSupply []*modules.TokenSupply
	tokenDefine *modules.TokenDefine
}

func NewRWSetBuilder() *RWSetBuilder {
	return &RWSetBuilder{make(map[string]*nsPubRwBuilder)}
}

func (b *RWSetBuilder) AddToReadSet(ns string, key string, version *modules.StateVersion) {
	nsPubRwBuilder := b.getOrCreateNsPubRwBuilder(ns)
	if nsPubRwBuilder.readMap == nil {
		nsPubRwBuilder.readMap = make(map[string]*KVRead)
	}
	// ReadSet
	nsPubRwBuilder.readMap[key] = NewKVRead(key, version)
}
func (b *RWSetBuilder) AddTokenPayOut(ns string, addr string, asset *modules.Asset, amount uint64, lockTime uint32) {
	nsPubRwBuilder := b.getOrCreateNsPubRwBuilder(ns)
	if nsPubRwBuilder.tokenPayOut == nil {
		nsPubRwBuilder.tokenPayOut = []*modules.TokenPayOut{}
	}
	address, _ := common.StringToAddress(addr)
	pay := &modules.TokenPayOut{Asset: asset, Amount: amount, PayTo: address, LockTime: lockTime}
	nsPubRwBuilder.tokenPayOut = append(nsPubRwBuilder.tokenPayOut, pay)

}
func (b *RWSetBuilder) AddToWriteSet(ns string, key string, value []byte) {
	nsPubRwBuilder := b.getOrCreateNsPubRwBuilder(ns)
	if nsPubRwBuilder.writeMap == nil {
		nsPubRwBuilder.writeMap = make(map[string]*KVWrite)
	}
	nsPubRwBuilder.writeMap[key] = newKVWrite(key, value)
}
func (b *RWSetBuilder) GetTokenPayOut(ns string) []*modules.TokenPayOut {
	nsPubRwBuilder := b.getOrCreateNsPubRwBuilder(ns)

	return nsPubRwBuilder.tokenPayOut
}
func (b *RWSetBuilder) GetTokenDefine(ns string) *modules.TokenDefine {
	nsPubRwBuilder := b.getOrCreateNsPubRwBuilder(ns)
	return nsPubRwBuilder.tokenDefine
}
func (b *RWSetBuilder) GetTokenSupply(ns string) []*modules.TokenSupply {
	nsPubRwBuilder := b.getOrCreateNsPubRwBuilder(ns)
	return nsPubRwBuilder.tokenSupply
}
func (b *RWSetBuilder) DefineToken(ns string, tokenType int32, define []byte, createAddr common.Address) {
	nsPubRwBuilder := b.getOrCreateNsPubRwBuilder(ns)
	nsPubRwBuilder.tokenDefine = &modules.TokenDefine{TokenType: int(tokenType), TokenDefineJson: define, Creator: createAddr}
}
func (b *RWSetBuilder) AddSupplyToken(ns string, assetId, uniqueId []byte, amt uint64, createAddr common.Address) error {
	nsPubRwBuilder := b.getOrCreateNsPubRwBuilder(ns)
	if nsPubRwBuilder.tokenSupply == nil {
		nsPubRwBuilder.tokenSupply = []*modules.TokenSupply{}
	}
	//TODO Devin 检查assetId，禁止创建PTN BTC ETH等系统定义的Token
	if bytes.Equal(assetId, modules.PTNCOIN.Bytes()) {
		return errors.New("Forbidden to supply System token PTN")
	}
	nsPubRwBuilder.tokenSupply = append(nsPubRwBuilder.tokenSupply, &modules.TokenSupply{AssetId: assetId,
		UniqueId: uniqueId, Amount: amt, Creator: createAddr})
	return nil
}

func (b *RWSetBuilder) getOrCreateNsPubRwBuilder(ns string) *nsPubRwBuilder {
	nsPubRwBuilder, ok := b.pubRwBuilderMap[ns]
	if !ok {
		nsPubRwBuilder = newNsPubRwBuilder(ns)
		b.pubRwBuilderMap[ns] = nsPubRwBuilder
		logger.Infof("**************,ns[%s], %v, %v", ns, nsPubRwBuilder, b.pubRwBuilderMap[ns])
	}
	return nsPubRwBuilder
}

func newNsPubRwBuilder(namespace string) *nsPubRwBuilder {
	return &nsPubRwBuilder{
		namespace,
		make(map[string]*KVRead),
		make(map[string]*KVWrite),
		[]*modules.TokenPayOut{},
		[]*modules.TokenSupply{},
		nil,
	}
}
