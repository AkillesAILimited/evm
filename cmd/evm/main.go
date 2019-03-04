package main

import (
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/mohanson/evm"
)

const help = `usage: evm <command> [<args>]

The most commonly used daze commands are:
  disasm     Disassemble bytecode
  exec       Execute bytecode
  create     Create a contract
  call       Call contract

Run 'evm <command> -h' for more information on a command.`

func printHelpAndExit() {
	fmt.Println(help)
	os.Exit(0)
}

func exDisasm() error {
	var (
		flCode = flag.String("code", "", "bytecode")
	)
	flag.Parse()
	code := common.FromHex(*flCode)
	for pc := 0; pc < len(code); pc++ {
		op := vm.OpCode(code[pc])
		fmt.Printf("[%04d] %v", pc, op)
		e := int(op)
		if e >= 0x60 && e <= 0x7F {
			l := e - int(vm.PUSH1) + 1
			off := pc + 1
			end := func() int {
				if len(code) < off+l {
					return len(code)
				}
				return off + l
			}()
			data := make([]byte, l)
			copy(data, code[off:end])
			fmt.Printf(" %#x", data)
			pc += l
		}
		fmt.Println()
	}
	return nil
}

func exInsert() error {
	var (
		flAddress = flag.String("address", "0x00", "")
		flBalance = flag.String("balance", "0x00", "")
		flCode    = flag.String("code", "0x", "")
		flDB      = flag.String("db", "db.json", "")
		flNonce   = flag.String("nonce", "0x00", "")
		flStorage = flag.String("storage", "", "k=v,k=v")
	)
	flag.Parse()
	d, err := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err != nil {
		return err
	}
	if err := evm.LoadStateDB(d, *flDB); err != nil {
		if os.IsExist(err) {
			return err
		}
	}
	a := common.HexToAddress(*flAddress)
	d.CreateAccount(a)
	b, _ := new(big.Int).SetString(*flBalance, 0)
	d.SetBalance(a, b)
	n, _ := new(big.Int).SetString(*flNonce, 0)
	d.SetNonce(a, n.Uint64())
	c := common.FromHex(*flCode)
	d.SetCode(a, c)
	if *flStorage != "" {
		for _, e := range strings.Split(*flStorage, ",") {
			seps := strings.Split(e, "=")
			k := seps[0]
			v := seps[1]
			d.SetState(a, common.HexToHash(k), common.HexToHash(v))
		}
	}
	return evm.SaveStateDB(d, *flDB)
}

func exMacall(subcmd string) error {
	var (
		flAddress     = flag.String("address", "", "")
		flBlockNumber = flag.String("number", "", "")
		flCode        = flag.String("code", "", "")
		flCoinbase    = flag.String("coinbase", "", "")
		flData        = flag.String("data", "", "")
		flDB          = flag.String("db", "db.json", "")
		flDifficulty  = flag.String("difficulty", "", "")
		flGasLimit    = flag.String("gaslimit", "100000", "")
		flGasPrice    = flag.String("gasprice", "1", "")
		flOrigin      = flag.String("origin", "", "")
		flValue       = flag.String("value", "", "")
	)
	flag.Parse()
	cfg := runtime.Config{}
	cfg.BlockNumber, _ = new(big.Int).SetString(*flBlockNumber, 0)
	cfg.Coinbase = common.HexToAddress(*flCoinbase)
	cfg.Difficulty, _ = new(big.Int).SetString(*flDifficulty, 0)
	cfg.GasLimit = func() uint64 {
		a, _ := new(big.Int).SetString(*flGasLimit, 0)
		return a.Uint64()
	}()
	cfg.GasPrice, _ = new(big.Int).SetString(*flGasPrice, 0)
	cfg.Origin = common.HexToAddress(*flOrigin)
	cfg.Value, _ = new(big.Int).SetString(*flValue, 0)
	cfg.EVMConfig.Debug = true
	slg := vm.NewStructLogger(nil)
	cfg.EVMConfig.Tracer = slg
	sdb, err := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err != nil {
		return err
	}
	if err := evm.LoadStateDB(sdb, *flDB); err != nil {
		if os.IsExist(err) {
			return err
		}
	}
	cfg.State = sdb
	switch subcmd {
	case "exec":
		ret, _, err := runtime.Execute(common.FromHex(*flCode), common.FromHex(*flData), &cfg)
		if err != nil {
			return err
		}
		vm.WriteTrace(os.Stdout, slg.StructLogs())
		fmt.Println()
		fmt.Println("Return  =", common.Bytes2Hex(ret))
		return nil
	case "create":
		_, add, gas, err := runtime.Create(common.FromHex(*flData), &cfg)
		if err != nil {
			return err
		}
		vm.WriteTrace(os.Stdout, slg.StructLogs())
		fmt.Println()
		fmt.Println("Cost    =", cfg.GasLimit-gas)
		fmt.Println("Address =", add.String())
		return evm.SaveStateDB(sdb, *flDB)
	case "call":
		ret, gas, err := runtime.Call(common.HexToAddress(*flAddress), common.FromHex(*flData), &cfg)
		if err != nil {
			return err
		}
		vm.WriteTrace(os.Stdout, slg.StructLogs())
		fmt.Println()
		fmt.Println("Cost    =", cfg.GasLimit-gas)
		fmt.Println("Return  =", common.Bytes2Hex(ret))
		return evm.SaveStateDB(sdb, *flDB)
	}
	return nil
}

func main() {
	if len(os.Args) <= 1 {
		printHelpAndExit()
	}
	subCommand := os.Args[1]
	os.Args = os.Args[1:len(os.Args)]
	var err error
	switch subCommand {
	case "disasm":
		err = exDisasm()
	case "insert":
		err = exInsert()
	case "exec", "create", "call":
		err = exMacall(subCommand)
	default:
		printHelpAndExit()
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
