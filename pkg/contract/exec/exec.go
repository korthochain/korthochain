package exec

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/contract/parser"
	"github.com/korthochain/korthochain/pkg/storage/merkle"
	"github.com/korthochain/korthochain/pkg/storage/store"
	"golang.org/x/crypto/sha3"
)

const (
	DEBUG = false
)

var dealRegistry map[string]scriptDealFunc

func init() {
	dealRegistry = make(map[string]scriptDealFunc)
	dealRegistry["new"] = deal0
	dealRegistry["mint"] = deal1
	dealRegistry["transfer"] = deal2
	dealRegistry["freeze"] = deal3
	dealRegistry["unfreeze"] = deal4
}

func TokenBalance(sdb *state.StateDB, id string, addr string) (uint64, error) {
	balance := sdb.GetBalance(common.HexToAddress(sHA1(string(bKey(id, addr)))))
	return balance.Uint64(), nil
}

func Precision(db store.DB, id string) (uint64, error) {
	v, err := db.Get(pKey(id))
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(v), nil
}

func TokenFreeze(sdb *state.StateDB, id string, addr string) (uint64, error) {
	balance := sdb.GetBalance(common.HexToAddress(sHA1(string(fKey(id, addr)))))
	return balance.Uint64(), nil
}

func New(db store.Transaction, sdb *state.StateDB, scs []*parser.Script, owner string) (*exec, error) {
	mp := make(map[string]string)
	for i, j := 0, len(scs); i < j; i++ {
		if scs[i] == nil {
			return nil, fmt.Errorf("parser script error")
		}
		if err := dealRegistry[scs[i].Name()](db, sdb, owner, mp, scs[i]); err != nil {
			return nil, err
		}
	}
	return &exec{db, sdb, mp}, nil
}

func (e *exec) Root() []byte {
	var ss []string
	var xs [][]byte

	for k, _ := range e.mp {
		ss = append(ss, k)
	}
	sort.Strings(ss)
	for _, k := range ss {
		xs = append(xs, []byte(k))
		xs = append(xs, []byte(e.mp[k]))
	}
	return merkle.New(sha3.New256(), xs).GetMtHash()
}

func (e *exec) Flush(tx store.Transaction, sdb *state.StateDB) error {
	for k, v := range e.mp {
		byteK, byteV, prefixB, prefixF, prefixT, prefixC := []byte(k), []byte(v), []byte("b/"), []byte("F/"), []byte("t/"), []byte("c/")

		if bytes.Equal(byteK[:2], prefixB) || bytes.Equal(byteK[:2], prefixF) || bytes.Equal(byteK[:2], prefixT) || bytes.Equal(byteK[:2], prefixC) {
			//set stateDB
			vBalance := new(big.Int).SetUint64(binary.LittleEndian.Uint64(byteV))
			sdb.SetBalance(common.HexToAddress(sHA1(k)), vBalance)
		} else {
			//set storeDB
			if err := tx.Set([]byte(k), []byte(v)); err != nil {
				return err
			}
		}
	}
	return nil
}

// new tokenId total_amount precision
func deal0(db store.Transaction, sdb *state.StateDB, executor string, mp map[string]string, sc *parser.Script) error {
	arg0, _ := sc.Arguments()[0].Value().(string) // tokenId
	arg1, _ := sc.Arguments()[1].Value().(uint64) // total_amount
	arg2, _ := sc.Arguments()[2].Value().(uint64) // precision
	{
		k := aKey(arg0)
		if _, ok := mp[string(k)]; ok {
			return errors.New("token exist")
		}
		if _, err := db.Get(k); err == nil {
			return errors.New("token exist")
		}
	}
	mp[arg0] = executor
	{
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, 0)
		mp[string(bKey(arg0, executor))] = string(buf)
	}
	{
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, arg1)
		mp[string(tKey(arg0, executor))] = string(buf)
	}
	{
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, arg2)
		mp[string(pKey(arg0))] = string(buf)
	}
	mp[string(fKey(arg0, executor))] = string(make([]byte, 8))
	return nil
}

func sdbGet(sdb *state.StateDB, arbKey []byte) (uint64, error) {
	balance := sdb.GetBalance(common.HexToAddress(sHA1(string(arbKey))))
	return balance.Uint64(), nil
}

// mint tokenId amount
func deal1(db store.Transaction, sdb *state.StateDB, executor string, mp map[string]string, sc *parser.Script) error {
	arg0, _ := sc.Arguments()[0].Value().(string) // tokenId
	arg1, _ := sc.Arguments()[1].Value().(uint64) // amount
	{
		v, err := db.Get(aKey(arg0))
		if err != nil {
			return err
		}
		if bytes.Compare(v, []byte(executor)) != 0 {
			return errors.New("permission denied")
		}
	}
	{
		var c, t uint64

		{
			k := tKey(arg0, executor)
			if v, ok := mp[string(k)]; ok {
				t = binary.LittleEndian.Uint64([]byte(v))
			} else {
				w, err := sdbGet(sdb, k)
				switch {
				case err == nil:
					t = w
				case err == store.NotExist:
					t = 0
				default:
					return err
				}
			}
		}
		k := cKey(arg0, executor)
		if v, ok := mp[string(k)]; ok {
			c = binary.LittleEndian.Uint64([]byte(v))
		} else {
			w, err := sdbGet(sdb, k)
			switch {
			case err == nil:
				c = w
			case err == store.NotExist:
				c = 0
			default:
				return err
			}
		}
		if int64(c) > int64(t)-int64(arg1) {
			return errors.New("overflow")
		}
		{
			buf := make([]byte, 8)
			binary.LittleEndian.PutUint64(buf, c+arg1)
			mp[string(cKey(arg0, executor))] = string(buf)
		}
	}
	var b uint64
	{
		k := bKey(arg0, executor)
		if v, ok := mp[string(k)]; ok {
			b = binary.LittleEndian.Uint64([]byte(v))
		} else {
			w, err := sdbGet(sdb, k)
			switch {
			case err == nil:
				b = w
			case err == store.NotExist:
				b = 0
			default:
				return err
			}
		}
	}
	{
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, b+arg1)
		mp[string(bKey(arg0, executor))] = string(buf)
	}
	return nil
}

// transfer tokenId amount address
func deal2(db store.Transaction, sdb *state.StateDB, executor string, mp map[string]string, sc *parser.Script) error {
	var from, to, free uint64

	arg0, _ := sc.Arguments()[0].Value().(string)  // tokenId
	arg1, _ := sc.Arguments()[1].Value().(uint64)  // amount
	arg2a, _ := sc.Arguments()[2].Value().(string) // address

	arg2b, err := address.NewAddrFromString(arg2a)
	if err != nil {
		return err
	}
	arg2c, err := arg2b.NewCommonAddr()
	if err != nil {
		return err
	}
	arg2 := arg2c.String()

	{
		k := aKey(arg0)
		if _, ok := mp[string(k)]; !ok {
			if _, err := db.Get(k); err != nil {
				return err
			}
		}
	}
	{
		k := fKey(arg0, executor)
		if v, ok := mp[string(k)]; ok {
			free = binary.LittleEndian.Uint64([]byte(v))
		} else {
			w, err := sdbGet(sdb, k)
			switch {
			case err == nil:
				free = w
			case err == store.NotExist:
				free = 0
			default:
				return err
			}
		}
	}
	{
		k := bKey(arg0, executor)
		if v, ok := mp[string(k)]; ok {
			from = binary.LittleEndian.Uint64([]byte(v))
		} else {
			w, err := sdbGet(sdb, k)
			switch {
			case err == nil:
				from = w
			case err == store.NotExist:
				from = 0
			default:
				return err
			}
		}
	}
	if from < arg1+free {
		return errors.New("insufficient balance")
	}
	{
		k := bKey(arg0, arg2)
		if v, ok := mp[string(k)]; ok {
			to = binary.LittleEndian.Uint64([]byte(v))
		} else {
			w, err := sdbGet(sdb, k)
			switch {
			case err == nil:
				to = w
			case err == store.NotExist:
				to = 0
			default:
				return err
			}
		}
	}
	if !bytes.Equal(bKey(arg0, arg2), bKey(arg0, executor)) {
		{
			buf := make([]byte, 8)
			binary.LittleEndian.PutUint64(buf, from-arg1)
			mp[string(bKey(arg0, executor))] = string(buf)
		}
		{
			buf := make([]byte, 8)
			binary.LittleEndian.PutUint64(buf, to+arg1)
			mp[string(bKey(arg0, arg2))] = string(buf)
		}
	}
	return nil
}

// freeze tokenId address amount
func deal3(db store.Transaction, sdb *state.StateDB, executor string, mp map[string]string, sc *parser.Script) error {
	arg0, _ := sc.Arguments()[0].Value().(string) // tokenId
	arg1, _ := sc.Arguments()[1].Value().(string) // address
	arg2, _ := sc.Arguments()[2].Value().(uint64) // amount
	{
		v, err := db.Get(aKey(arg0))
		if err != nil {
			return err
		}
		if bytes.Compare(v, []byte(executor)) != 0 {
			return errors.New("permission denied")
		}
	}
	var b, f uint64
	{
		k := bKey(arg0, arg1)
		if v, ok := mp[string(k)]; ok {
			b = binary.LittleEndian.Uint64([]byte(v))
		} else {
			w, err := sdbGet(sdb, k)
			switch {
			case err == nil:
				b = w
			case err == store.NotExist:
				b = 0
			default:
				return err
			}
		}
	}

	{
		k := fKey(arg0, arg1)
		if v, ok := mp[string(k)]; ok {
			f = binary.LittleEndian.Uint64([]byte(v))
		} else {
			w, err := sdbGet(sdb, k)
			switch {
			case err == nil:
				f = w
			case err == store.NotExist:
				f = 0
			default:
				return err
			}
		}
		buf := make([]byte, 8)
		f += arg2
		if f > b {
			f = b
		}
		binary.LittleEndian.PutUint64(buf, f)
		mp[string(k)] = string(buf)
	}
	return nil
}

// unfreeze tokenId address amount
func deal4(db store.Transaction, sdb *state.StateDB, executor string, mp map[string]string, sc *parser.Script) error {
	arg0, _ := sc.Arguments()[0].Value().(string) // tokenId
	arg1, _ := sc.Arguments()[1].Value().(string) // address
	arg2, _ := sc.Arguments()[2].Value().(uint64) // amount
	{
		v, err := db.Get(aKey(arg0))
		if err != nil {
			return err
		}
		if bytes.Compare(v, []byte(executor)) != 0 {
			return errors.New("permission denied")
		}
	}
	{
		var t uint64

		k := fKey(arg0, arg1)
		if v, ok := mp[string(k)]; ok {
			t = binary.LittleEndian.Uint64([]byte(v))
		} else {
			w, err := sdbGet(sdb, k)
			switch {
			case err == nil:
				t = w
			case err == store.NotExist:
				t = 0
			default:
				return err
			}
		}
		buf := make([]byte, 8)
		if t < arg2 {
			t = 0
		} else {
			t = t - arg2
		}
		binary.LittleEndian.PutUint64(buf, t)
		mp[string(k)] = string(buf)
	}
	return nil
}

func aKey(id string) []byte {
	return []byte(id)
}

func bKey(id, addr string) []byte {
	var buf bytes.Buffer

	buf.WriteString("b")
	buf.WriteString("/")
	buf.WriteString(id)
	buf.WriteString(addr)
	return buf.Bytes()
}

func pKey(id string) []byte {
	var buf bytes.Buffer

	buf.WriteString("p")
	buf.WriteString("/")
	buf.WriteString(id)
	return buf.Bytes()
}

func fKey(id, addr string) []byte {
	var buf bytes.Buffer

	buf.WriteString("F")
	buf.WriteString("/")
	buf.WriteString(id)
	buf.WriteString(addr)
	return buf.Bytes()
}

func tKey(id, addr string) []byte {
	var buf bytes.Buffer

	buf.WriteString("t")
	buf.WriteString("/")
	buf.WriteString(id)
	buf.WriteString(addr)
	return buf.Bytes()
}

func cKey(id, addr string) []byte {
	var buf bytes.Buffer

	buf.WriteString("c")
	buf.WriteString("/")
	buf.WriteString(id)
	buf.WriteString(addr)
	return buf.Bytes()
}

func sHA1(s string) string {
	o := sha1.New()
	o.Write([]byte(s))
	return hex.EncodeToString(o.Sum(nil))
}
