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
 *  * @author PalletOne core developers <dev@pallet.one>
 *  * @date 2018
 *
 */

package ptnapi

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	"github.com/palletone/go-palletone/common"

	"github.com/palletone/go-palletone/common/hexutil"
	"github.com/palletone/go-palletone/common/math"
	"github.com/palletone/go-palletone/core/accounts"
	"github.com/palletone/go-palletone/core/accounts/keystore"
)

// PrivateAccountAPI provides an API to access accounts managed by this node.
// It offers methods to create, (un)lock en list accounts. Some methods accept
// passwords and are therefore considered private by default.
type PrivateAccountAPI struct {
	am        *accounts.Manager
	nonceLock *AddrLocker
	b         Backend
}

// NewPrivateAccountAPI create a new PrivateAccountAPI.
func NewPrivateAccountAPI(b Backend, nonceLock *AddrLocker) *PrivateAccountAPI {
	return &PrivateAccountAPI{
		am:        b.AccountManager(),
		nonceLock: nonceLock,
		b:         b,
	}
}

// ListAccounts will return a list of addresses for accounts this node manages.
func (s *PrivateAccountAPI) ListAccounts() []string {
	addresses := make([]string, 0)
	for _, wallet := range s.am.Wallets() {
		for _, account := range wallet.Accounts() {
			addresses = append(addresses, account.Address.String())
		}
	}
	return addresses
}

// rawWallet is a JSON representation of an accounts.Wallet interface, with its
// data contents extracted into plain fields.
type rawWallet struct {
	URL      string             `json:"url"`
	Status   string             `json:"status"`
	Failure  string             `json:"failure,omitempty"`
	Accounts []accounts.Account `json:"accounts,omitempty"`
}

// ListWallets will return a list of wallets this node manages.
func (s *PrivateAccountAPI) ListWallets() []rawWallet {
	wallets := make([]rawWallet, 0) // return [] instead of nil if empty
	for _, wallet := range s.am.Wallets() {
		status, failure := wallet.Status()

		raw := rawWallet{
			URL:      wallet.URL().String(),
			Status:   status,
			Accounts: wallet.Accounts(),
		}
		if failure != nil {
			raw.Failure = failure.Error()
		}
		wallets = append(wallets, raw)
	}
	return wallets
}

// OpenWallet initiates a hardware wallet opening procedure, establishing a USB
// connection and attempting to authenticate via the provided passphrase. Note,
// the method may return an extra challenge requiring a second open (e.g. the
// Trezor PIN matrix challenge).
func (s *PrivateAccountAPI) OpenWallet(url string, passphrase *string) error {
	wallet, err := s.am.Wallet(url)
	if err != nil {
		return err
	}
	pass := ""
	if passphrase != nil {
		pass = *passphrase
	}
	return wallet.Open(pass)
}

// DeriveAccount requests a HD wallet to derive a new account, optionally pinning
// it for later reuse.
func (s *PrivateAccountAPI) DeriveAccount(url string, path string, pin *bool) (accounts.Account, error) {
	wallet, err := s.am.Wallet(url)
	if err != nil {
		return accounts.Account{}, err
	}
	derivPath, err := accounts.ParseDerivationPath(path)
	if err != nil {
		return accounts.Account{}, err
	}
	if pin == nil {
		pin = new(bool)
	}
	return wallet.Derive(derivPath, *pin)
}

// NewAccount will create a new account and returns the address for the new account.
func (s *PrivateAccountAPI) NewAccount(password string) (string, error) {
	acc, err := fetchKeystore(s.am).NewAccount(password)
	if err == nil {
		return acc.Address.String(), nil
	}
	return "ERROR", err
}

func (s *PrivateAccountAPI) NewOutAccount(password string) (string, error) {
	acc, err := fetchKeystore(s.am).NewAccountOutchain(password)
	if err == nil {
		return acc.Address.String(), nil
	}
	return "ERROR", err
}

type NewHdAccountResult struct {
	Address  common.Address
	Mnemonic string
}

func (s *PrivateAccountAPI) NewHdAccount(password string) (*NewHdAccountResult, error) {
	acc, mnemonic, err := fetchKeystore(s.am).NewHdAccount(password)
	if err != nil {
		return nil, err
	}
	return &NewHdAccountResult{Address: acc.Address, Mnemonic: mnemonic}, nil
}

func (s *PrivateAccountAPI) GetHdAccount(addr, password string, userId Int) (string, error) {
	_, err := common.StringToAddress(addr)
	if err != nil {
		return "", err
	}
	account, _ := MakeAddress(fetchKeystore(s.am), addr)
	accountIndex := userId.Uint32()
	if err != nil {
		return "", errors.New("invalid argument, args 2 must be a number")
	}
	ks := fetchKeystore(s.am)
	var acc accounts.Account
	if ks.IsUnlock(account.Address) {
		acc, err = ks.GetHdAccount(account, accountIndex)
	} else {
		acc, err = ks.GetHdAccountWithPassphrase(account, password, accountIndex)
	}
	if err != nil {
		return "", err
	}
	return acc.Address.String(), nil
}
func (s *PrivateAccountAPI) IsUnlock(addrStr string) (bool, error) {
	ks := s.b.GetKeyStore()
	addr, err := common.StringToAddress(addrStr)
	if err != nil {
		return false, err
	}
	return ks.IsUnlock(addr), nil
}

// fetchKeystore retrives the encrypted keystore from the account manager.
func fetchKeystore(am *accounts.Manager) *keystore.KeyStore {
	return am.Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
}

// ImportRawKey stores the given hex encoded ECDSA key into the key directory,
// encrypting it with the passphrase.
func (s *PrivateAccountAPI) ImportRawKey(privkey string, password string) (string, error) {
	key, err := hexutil.Decode(privkey)
	if err != nil {
		return "", err
	}
	acc, err := fetchKeystore(s.am).ImportECDSA(key, password)
	return acc.Address.String(), err
}
func (s *PrivateAccountAPI) ImportMnemonic(mnemonic string, password string) (string, error) {
	acc, err := fetchKeystore(s.am).ImportMnemonic(mnemonic, password)
	return acc.Address.String(), err
}
func (s *PrivateAccountAPI) ImportHdAccountMnemonic(mnemonic string, password string) (string, error) {
	acc, err := fetchKeystore(s.am).ImportHdSeedFromMnemonic(mnemonic, password)
	return acc.Address.String(), err
}

// UnlockAccount will unlock the account associated with the given address with
// the given password for duration seconds. If duration is nil it will use a
// default of 300 seconds. It returns an indication if the account was unlocked.
func (s *PrivateAccountAPI) UnlockAccount(addrStr string, password string, duration *uint64) (bool, error) {
	addr, _ := common.StringToAddress(addrStr)
	const max = uint64(time.Duration(math.MaxInt64) / time.Second)
	var d time.Duration
	if duration == nil {
		d = 300 * time.Second
	} else if *duration > max {
		return false, errors.New("unlock duration too large")
	} else {
		d = time.Duration(*duration) * time.Second
	}
	err := fetchKeystore(s.am).TimedUnlock(accounts.Account{Address: addr}, password, d)
	return err == nil, err
}

// LockAccount will lock the account associated with the given address when it's unlocked.
func (s *PrivateAccountAPI) LockAccount(addrStr string) bool {
	addr, _ := common.StringToAddress(addrStr)
	return fetchKeystore(s.am).Lock(addr) == nil
}

//对一个文本进行签名
func (s *PrivateAccountAPI) Sign(ctx context.Context, data string, addr string,
	passwd string) (hexutil.Bytes, error) {
	bytes := []byte(data)
	return s.SignHex(ctx, bytes, addr, passwd)
}

//对16进制数据进行签名
func (s *PrivateAccountAPI) SignHex(ctx context.Context, data hexutil.Bytes, addr string,
	passwd string) (hexutil.Bytes, error) {
	// Look up the wallet containing the requested signer
	address, _ := common.StringToAddress(addr)
	account := accounts.Account{Address: address}

	wallet, err := s.b.AccountManager().Find(account)
	if err != nil {
		return nil, err
	}
	// Assemble sign the data with the wallet
	signature, err := wallet.SignMessageWithPassphrase(account, passwd, data)
	if err != nil {
		return nil, err
	}

	return signature, nil
}

// appended by albert·gou
//func (s *PrivateAccountAPI) TransferPtn(from, to string, amount decimal.Decimal, text *string,
//	password string) (*TxExecuteResult, error) {
//	// 参数检查
//	fromAdd, err := common.StringToAddress(from)
//	if err != nil {
//		return nil, fmt.Errorf("invalid account address: %v", from)
//	}
//
//	// 解锁账户
//	ks := fetchKeystore(s.am)
//	if !ks.IsUnlock(fromAdd) {
//		duration := 1 * time.Second
//		err = ks.TimedUnlock(accounts.Account{Address: fromAdd}, password, duration)
//		if err != nil {
//			return nil, err
//		}
//	}
//
//	return s.b.TransferPtn(from, to, amount, text)
//}
func (s *PrivateAccountAPI) GetPublicKey(address string, password string) (string, error) {
	addr, err := common.StringToAddress(address)
	if err != nil {
		return "", err
	}
	ks := s.b.GetKeyStore()
	if !ks.IsUnlock(addr) {
		err = ks.Unlock(accounts.Account{Address: addr}, password)
		if err != nil {
			return "", err
		}
	}
	byte, err := ks.GetPublicKey(addr)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(byte), nil
}

func (s *PrivateAccountAPI) DumpPrivateKey(address string, password string) (string, error) {
	addr, err := common.StringToAddress(address)
	if err != nil {
		return "", err
	}
	ks := s.b.GetKeyStore()
	byte, _, err := ks.DumpKey(accounts.Account{Address: addr}, password)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(byte), nil
}
func (s *PrivateAccountAPI) ConvertAccount(address string) (string, error) {
	addr, err := common.StringToAddress(address)
	if err != nil {
		return "", err
	}
	return hexutil.Encode(addr.Bytes()), nil
}
