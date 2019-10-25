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
 * @author PalletOne core developer Albert·Gou <dev@pallet.one>
 * @date 2018
 */

package mediatorplugin

import (
	"fmt"
	"strconv"
	"time"

	"github.com/palletone/go-palletone/common"
	"github.com/palletone/go-palletone/common/event"
	"github.com/palletone/go-palletone/common/log"
	"github.com/palletone/go-palletone/core"
	"github.com/palletone/go-palletone/dag/rwset"
)

func (mp *MediatorPlugin) newChainBanner() {
	//log.Infof("\n" +
	//	"*   ------- NEW CHAIN -------   *\n" +
	//	"*   - Welcome to PalletOne! -   *\n" +
	//	"*   -------------------------   *\n" +
	//	"\n")
	log.Info("welcome PalletOne new chain")

	if mp.dag.GetSlotAtTime(time.Now()) > 200 {
		log.Debugf("Your genesis seems to have an old timestamp. " +
			"Please consider using the --genesistime option to give your genesis a recent timestamp.")
	}
}

func (mp *MediatorPlugin) SubscribeNewProducedUnitEvent(ch chan<- NewProducedUnitEvent) event.Subscription {
	return mp.newProducedUnitScope.Track(mp.newProducedUnitFeed.Subscribe(ch))
}

func (mp *MediatorPlugin) scheduleProductionLoop() {
	// 1. 计算下一秒的滴答时刻，如果少于50毫秒，则多等一秒开始
	now := time.Now()
	timeToNextSecond := time.Second - time.Duration(now.Nanosecond())
	if timeToNextSecond < 50*time.Millisecond {
		timeToNextSecond += time.Second
	}

	// 2. 安排unit生产循环
	// Start to production unit for expiration
	timeout := time.NewTimer(timeToNextSecond)
	defer timeout.Stop()

	// production unit until termination is requested
	select {
	case <-mp.quit:
		return
	case <-mp.stopProduce:
		return
	case <-timeout.C:
		go mp.unitProductionLoop()
	}
}

// unit生产的状态类型
type ProductionCondition uint8

// unit生产的状态枚举
const (
	Produced ProductionCondition = iota // 正常生产unit
	NotSynced
	NotMyTurn
	NotTimeYet
	NotUnlocked
	LowParticipation
	Lag
	Consecutive
	ExceptionProducing
)

func (mp *MediatorPlugin) unitProductionLoop() ProductionCondition {
	mp.wg.Add(1)
	defer mp.wg.Done()

	// 1. 尝试生产unit
	result, detail := mp.maybeProduceUnit()

	// 2. 打印尝试结果
	switch result {
	case Produced:
		log.Infof("Generated unit(%v) #%v parent(%v) @%v signed by %v", detail["Hash"], detail["Num"],
			detail["ParentHash"], detail["Timestamp"], detail["Mediator"])
	case NotSynced:
		log.Infof("Not producing unit because production is disabled until we receive a recent unit." +
			" Disable this check with --staleProduce option.")
	case NotTimeYet:
		log.Debugf("Not producing unit because next slot time is %v , but now is %v",
			detail["NextTime"], detail["Now"])
	case NotMyTurn:
		log.Debugf("Not producing unit because current scheduled mediator is %v", detail["ScheduledMediator"])
	case Lag:
		log.Infof("Not producing unit because node didn't wake up within 2500ms of the slot time."+
			" Scheduled Time is: %v , but now is %v", detail["ScheduledTime"], detail["Now"])
	case NotUnlocked:
		log.Infof("Not producing unit because we are not unlocked for account: %v", detail["ScheduledKey"])
	case Consecutive:
		log.Infof("Not producing unit because the last unit was generated by the same mediator(%v)."+
			" This node is probably disconnected from the network so unit production has been disabled."+
			" Disable this check with --allowConsecutive option.", detail["Mediator"])
	case LowParticipation:
		log.Infof("Not producing unit because node appears to be on a minority fork with only %v "+
			"mediator participation.", detail["ParticipationRate"])
	case ExceptionProducing:
		log.Infof("Exception producing unit: %v", detail["Msg"])
	default:
		log.Infof("Unknown condition when producing unit!")
	}

	// 3. 继续循环生产计划
	go mp.scheduleProductionLoop()

	return result
}

func (mp *MediatorPlugin) maybeProduceUnit() (ProductionCondition, map[string]string) {
	detail := make(map[string]string)
	dag := mp.dag

	// 整秒调整，四舍五入
	nowFine := time.Now()
	now := time.Unix(nowFine.Add(500*time.Millisecond).Unix(), 0)

	// 1. 判断是否满足生产的各个条件
	nextSlotTime := dag.GetSlotTime(1)
	// If the next Unit production opportunity is in the present or future, we're synced.
	if !mp.productionEnabled {
		if nextSlotTime.After(now) || nextSlotTime.Equal(now) {
			mp.productionEnabled = true
		} else {
			return NotSynced, detail
		}
	}

	slot := dag.GetSlotAtTime(now)
	// is anyone scheduled to produce now or one second in the future?
	if slot == 0 {
		detail["NextTime"] = nextSlotTime.Format("2006-01-02 15:04:05.000")
		detail["Now"] = now.Format("2006-01-02 15:04:05.000")
		return NotTimeYet, detail
	}

	// this conditional judgment should fail, because now <= HeadUnitTime
	// should have resulted in slot == 0.
	//
	// if this assert triggers, there is a serious bug in dag.GetSlotAtTime()
	// which would result in allowing a later unit to have a timestamp
	// less than or equal to the previous unit
	if !(now.Unix() > dag.HeadUnitTime()) {
		detail["Msg"] = "The property database is being updated because the new unit is received synchronously."
		return ExceptionProducing, detail
	}

	scheduledMediator := dag.GetScheduledMediator(slot)
	if scheduledMediator.Equal(common.Address{}) {
		detail["Msg"] = "The current shuffled mediators is nil!"
		return ExceptionProducing, detail
	}

	// we must control the Mediator scheduled to produce the next Unit.
	//med, ok := mp.mediators[scheduledMediator]
	_, ok := mp.mediators[scheduledMediator]
	if !ok {
		detail["ScheduledMediator"] = scheduledMediator.Str()
		return NotMyTurn, detail
	}

	// 判断scheduledMediator账户在keystore中是否解锁
	ks := mp.ptn.GetKeyStore()
	if !ks.IsUnlock(scheduledMediator) {
		detail["ScheduledKey"] = scheduledMediator.Str()
		return NotUnlocked, detail
	}

	if dag.IsConsecutiveMediator(scheduledMediator) {
		if mp.consecutiveProduceEnabled {
			// 连续产块的特权只能使用一次
			mp.consecutiveProduceEnabled = false
		} else {
			detail["Mediator"] = scheduledMediator.Str()
			return Consecutive, detail
		}
	}

	pRate := dag.MediatorParticipationRate()
	if pRate < mp.requiredParticipation {
		detail["ParticipationRate"] = fmt.Sprint(pRate / core.PalletOne1Percent)
		return LowParticipation, detail
	}

	// todo 由于当前代码更新数据库没有加锁，可能如下情况：
	// 生产单元的协程满足了前面的判断，此时新收到一个unit正在更新数据库，后面的判断有不能通过
	scheduledTime := dag.GetSlotTime(slot)
	diff := scheduledTime.Sub(now)
	if diff > 2500*time.Millisecond || diff < -2500*time.Millisecond {
		detail["ScheduledTime"] = scheduledTime.Format("2006-01-02 15:04:05")
		detail["Now"] = now.Format("2006-01-02 15:04:05")
		return Lag, detail
	}

	// todo 待调整处理逻辑
	// 重置rwManager
	rwset.Init()
	// execute contract
	if err := mp.ptn.ContractProcessor().AddContractLoop(rwset.RwM, mp.ptn.TxPool(), scheduledMediator, ks); err != nil {
		log.Debugf("MaybeProduceUnit RunContractLoop err: %v", err.Error())
	}
	// close tx simulator (系统合约)
	rwset.RwM.CloseTxSimulator(rwset.ChainId, "")
	rwset.RwM.Close()

	//广播节点选取签名请求事件
	go mp.ptn.ContractProcessor().BroadcastElectionSigRequestEvent()

	// 2. 生产单元
	var groupPubKey []byte = nil
	if mp.groupSigningEnabled {
		groupPubKey = mp.localMediatorPubKey(scheduledMediator)
		if len(groupPubKey) == 0 {
			log.Debugf("the groupPubKey is nil")
		}
	}

	newUnit, err := dag.GenerateUnit(scheduledTime, scheduledMediator, groupPubKey, ks, mp.ptn.TxPool())
	if err != nil {
		detail["Msg"] = fmt.Sprintf("GenerateUnit err: %v", err.Error())
		return ExceptionProducing, detail
	}

	unitHash := newUnit.UnitHash
	detail["Num"] = strconv.FormatUint(newUnit.NumberU64(), 10)
	time := time.Unix(newUnit.Timestamp(), 0)
	detail["Timestamp"] = time.Format("2006-01-02 15:04:05")
	detail["Mediator"] = scheduledMediator.Str()
	detail["Hash"] = unitHash.TerminalString()
	detail["ParentHash"] = newUnit.ParentHash()[0].TerminalString()

	// 3. 对 unit 进行群签名和广播
	go mp.groupSignUnit(scheduledMediator, unitHash)

	// 4. 异步向区块链网络广播新unit
	go mp.newProducedUnitFeed.Send(NewProducedUnitEvent{Unit: newUnit})

	return Produced, detail
}

func (mp *MediatorPlugin) groupSignUnit(localMed common.Address, unitHash common.Hash) {
	if !mp.groupSigningEnabled {
		return
	}

	// 1. 初始化签名unit相关的签名分片的buf
	mp.toTBLSBufLock.Lock()
	if _, ok := mp.toTBLSRecoverBuf[localMed]; !ok {
		mp.toTBLSRecoverBuf[localMed] = make(map[common.Hash]*sigShareSet)
	}
	aSize := mp.dag.ActiveMediatorsCount()
	mp.toTBLSRecoverBuf[localMed][unitHash] = newSigShareSet(aSize)
	mp.toTBLSBufLock.Unlock()

	// 2. 过了 unit 确认时间后，及时删除群签名分片的相关数据，防止内存溢出
	go func() {
		expiration := mp.dag.UnitIrreversibleTime()
		deleteBuf := time.NewTimer(expiration)

		select {
		case <-mp.quit:
			return
		case <-deleteBuf.C:
			mp.toTBLSBufLock.Lock()
			if _, ok := mp.toTBLSRecoverBuf[localMed][unitHash]; ok {
				log.Debugf("the unit(%v) has expired confirmation time, no longer need the mediator(%v) "+
					"to recover group-sign", unitHash.TerminalString(), localMed.Str())
				delete(mp.toTBLSRecoverBuf[localMed], unitHash)
			}
			mp.toTBLSBufLock.Unlock()
		}
	}()
}
