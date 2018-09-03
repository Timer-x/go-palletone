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
 *
 */

package storage

import (
	"fmt"

	"github.com/palletone/go-palletone/common/log"
	"github.com/palletone/go-palletone/dag/modules"
	"github.com/palletone/go-palletone/common/ptndb"
)

const (
	globalPropDBKey    = "GlobalProperty"
	dynGlobalPropDBKey = "DynamicGlobalProperty"
)

//type globalProperty struct {
//	ChainParameters core.ChainParameters
//
//	ActiveMediators []core.MediatorInfo
//}
//
//func getInfoFromMediator(m core.Mediator) core.MediatorInfo {
//
//}
//
//func getGlobalProperty(gp *modules.GlobalProperty) globalProperty {
//	gpt := globalProperty{ChainParameters: gp.ChainParameters}
//	for _, medInfo := range gp.ActiveMediators{
//
//	}
//
//	return gpt
//}

func StoreGlobalProp(db ptndb.Database,gp *modules.GlobalProperty) {


//	gpt :=

	err := Store(db, globalPropDBKey, *gp)
	if err != nil {
		log.Error(fmt.Sprintf("Store global properties error:%s", err))
	}
}

func StoreDynGlobalProp(db ptndb.Database,dgp *modules.DynamicGlobalProperty) {

	err := Store(db, dynGlobalPropDBKey, *dgp)
	if err != nil {
		log.Error(fmt.Sprintf("Store dynamic global properties error: %s", err))
	}
}

func RetrieveGlobalProp(db ptndb.Database) *modules.GlobalProperty {
	gp := modules.NewGlobalProp()

	err := Retrieve(db,globalPropDBKey, gp)
	if err != nil {
		log.Error(fmt.Sprintf("Retrieve global properties error: %s", err))
	}

	return gp
}

func RetrieveDynGlobalProp(db ptndb.Database) *modules.DynamicGlobalProperty {
	dgp := modules.NewDynGlobalProp()

	err := Retrieve(db,dynGlobalPropDBKey, dgp)
	if err != nil {
		log.Error(fmt.Sprintf("Retrieve dynamic global properties error: %s", err))
	}

	return dgp
}
