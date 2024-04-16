package main

import (
	"bytes"
	"context"
	"fmt"
	"math/big"

	amt4 "github.com/filecoin-project/go-amt-ipld/v4"
	"github.com/ipfs/go-cid"
	"github.com/urfave/cli/v2"
	cbg "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"

	ethabi "github.com/defiweb/go-eth/abi"
	goethtypes "github.com/defiweb/go-eth/types"
	"github.com/filecoin-project/go-address"
	builtintypes "github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/lotus/api/v0api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/types/ethtypes"
	lcli "github.com/filecoin-project/lotus/cli"
)

type apiIpldStore struct {
	ctx context.Context
	api v0api.FullNode
}

func (ht *apiIpldStore) Context() context.Context {
	return ht.ctx
}

func (ht *apiIpldStore) Get(ctx context.Context, c cid.Cid, out interface{}) error {
	raw, err := ht.api.ChainReadObj(ctx, c)
	if err != nil {
		return err
	}

	cu, ok := out.(cbg.CBORUnmarshaler)
	if ok {
		if err := cu.UnmarshalCBOR(bytes.NewReader(raw)); err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("Object does not implement CBORUnmarshaler")
}

func (ht *apiIpldStore) Put(ctx context.Context, v interface{}) (cid.Cid, error) {
	panic("No mutations allowed")
}

var EmptyEthAddress  = ethtypes.EthAddress{}

var FilplusDeployContractCmd = &cli.Command{
	Name:      "deploy-allocator-contract",
	Usage:     "deploy allocator contract",
	ArgsUsage: "registryAddress initialContractOwner",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "from",
			Usage:    "optionally specify your address to send the message from",
		},
	},
	Action: func(cctx *cli.Context) error {
		afmt := lcli.NewAppFmt(cctx.App)

		if cctx.NArg() != 2 {
			return lcli.IncorrectNumArgs(cctx)
		}

		api, closer, err := lcli.GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := lcli.ReqContext(cctx)

		var from address.Address
		if froms := cctx.String("from"); froms == "" {
			from, err = api.WalletDefaultAddress(ctx)
		} else {
			from, err = address.NewFromString(froms)
		}
		if err != nil {
			return err
		}

		registryAddress, err := ethtypes.ParseEthAddress(cctx.Args().Get(0))
		if err != nil {
			return err
		}

		fRegistryAddress, err := registryAddress.ToFilecoinAddress()
		if err != nil {
			return err
		}

		initialContractOwner, err := ethtypes.ParseEthAddress(cctx.Args().Get(1))
		if err != nil {
			return err
		}


		deploy := ethabi.MustParseMethod("deploy(address)");
		calldata := deploy.MustEncodeArgs(initialContractOwner);
		if err != nil {
			return err
		}

		var buffer bytes.Buffer
		if err := cbg.WriteByteArray(&buffer, calldata); err != nil {
			return xerrors.Errorf("failed to encode evm params as cbor: %w", err)
		}
		calldata = buffer.Bytes()

		msg := &types.Message{
			To:     fRegistryAddress,
			From:   from,
			Method: builtintypes.MethodsEVM.InvokeContract,
			Params: calldata,
		}
    // event Deployed(address deployedContractAddress);

		afmt.Println("sending message...")
		smsg, err := api.MpoolPushMessage(ctx, msg, nil)
		if err != nil {
			return err
		}
		afmt.Println("waiting for message to execute...")
		wait, err := api.StateWaitMsg(ctx, smsg.Cid(), 0)
		if err != nil {
			return err
		}

		if wait.Receipt.ExitCode != 0 {
			return xerrors.Errorf("actor execution failed")
		}

		if eventsRoot := wait.Receipt.EventsRoot; eventsRoot != nil {
			afmt.Println("Events emitted:")

			s := &apiIpldStore{ctx, api}
			amt, err := amt4.LoadAMT(ctx, s, *eventsRoot, amt4.UseTreeBitWidth(types.EventAMTBitwidth))
			if err != nil {
				return err
			}

			var evt types.Event
			err = amt.ForEach(ctx, func(u uint64, deferred *cbg.Deferred) error {
				fmt.Printf("%x\n", deferred.Raw)
				if err := evt.UnmarshalCBOR(bytes.NewReader(deferred.Raw)); err != nil {
					return err
				}
				if err != nil {
					return err
				}
				fmt.Printf("\tEmitter ID: %s\n", evt.Emitter)
				for _, e := range evt.Entries {
					value, err := cbg.ReadByteArray(bytes.NewBuffer(e.Value), uint64(len(e.Value)))
					if err != nil {
						return err
					}
					fmt.Printf("\t\tKey: %s, Value: 0x%x, Flags: b%b\n", e.Key, value, e.Flags)
				}
				return nil

			})
		}

		afmt.Println("OK")

		return nil
	},
}
var FilplusListContractsCmd = &cli.Command{
	Name:      "list-contracts",
	Usage:     "list registered allocator contracts",
	ArgsUsage: "registryAddress",
	Flags:     []cli.Flag{},
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return lcli.IncorrectNumArgs(cctx)
		}

		registryAddress, err := ethtypes.ParseEthAddress(cctx.Args().Get(0))
		if err != nil {
			return err
		}

		contracts := ethabi.MustParseMethod("contracts() returns (address[])");

		api, closer, err := lcli.GetFullNodeAPIV1(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := lcli.ReqContext(cctx)
		res, err := api.EthCall(ctx, ethtypes.EthCall{
			From: &EmptyEthAddress,
			To:   &registryAddress,
			Data: contracts.MustEncodeArgs(),
		}, ethtypes.NewEthBlockNumberOrHashFromPredefined("latest"))
		if err != nil {
			fmt.Println("Eth call fails, return val: ", res)
			return err
		}

		var addresses []goethtypes.Address;
		contracts.MustDecodeValues(res, &addresses);

		fmt.Println(addresses)
		return nil
	},
}

var FilplusListAllocatorsCmd = &cli.Command{
	Name:      "list-allocators",
	Usage:     "list allocators in given allocator contract",
	ArgsUsage: "allocatorContractAddress",
	Flags:     []cli.Flag{},
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return lcli.IncorrectNumArgs(cctx)
		}

		contractAddress, err := ethtypes.ParseEthAddress(cctx.Args().Get(0))
		if err != nil {
			return err
		}

		allocators := ethabi.MustParseMethod("allocators() returns (address[])");
		allowance := ethabi.MustParseMethod("allowance(address) returns (uint256)");

		api, closer, err := lcli.GetFullNodeAPIV1(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := lcli.ReqContext(cctx)
		res, err := api.EthCall(ctx, ethtypes.EthCall{
			From: &EmptyEthAddress,
			To:   &contractAddress,
			Data: allocators.MustEncodeArgs(),
		}, ethtypes.NewEthBlockNumberOrHashFromPredefined("latest"))
		if err != nil {
			fmt.Println("Eth call fails, return val: ", res)
			return err
		}

		var addresses []goethtypes.Address;
		allocators.MustDecodeValues(res, &addresses);

		for i := 0; i < len(addresses); i++ { 
			address := addresses[i]
			res, err := api.EthCall(ctx, ethtypes.EthCall{
				From: &EmptyEthAddress,
				To:   &contractAddress,
				Data: allowance.MustEncodeArgs(address),
			}, ethtypes.NewEthBlockNumberOrHashFromPredefined("latest"))
			if err != nil {
				fmt.Println("Eth call fails, return val: ", res)
				return err
			}
			var allowanceAmount *big.Int;
			allowance.MustDecodeValues(res, &allowanceAmount);
			fmt.Println(address, allowanceAmount);
		} 
		return nil
	},
}

var FilplusAddAllowanceCmd = &cli.Command{
	Name:      "add-allowance",
	Usage:     "grant allowance to allocator",
	ArgsUsage: "allocatorContractAddress allocatorAddress amount",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "from",
			Usage:    "optionally specify your address to send the message from",
		},
	},
	Action: func(cctx *cli.Context) error {
		afmt := lcli.NewAppFmt(cctx.App)

		if cctx.NArg() != 3 {
			return lcli.IncorrectNumArgs(cctx)
		}

		api, closer, err := lcli.GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := lcli.ReqContext(cctx)

		var from address.Address
		if froms := cctx.String("from"); froms == "" {
			from, err = api.WalletDefaultAddress(ctx)
		} else {
			from, err = address.NewFromString(froms)
		}
		if err != nil {
			return err
		}

		contractAddress, err := ethtypes.ParseEthAddress(cctx.Args().Get(0))
		if err != nil {
			return err
		}
		fContractAddress, err := contractAddress.ToFilecoinAddress()
		if err != nil {
			return err
		}

		allocatorAddress, err := ethtypes.ParseEthAddress(cctx.Args().Get(1))
		if err != nil {
			return err
		}

		amount := new(big.Int)
		_, err = fmt.Sscan(cctx.Args().Get(2), amount)
		if err != nil {
			return err
		}

		addAllowance := ethabi.MustParseMethod("addAllowance(address,uint256)");
		calldata := addAllowance.MustEncodeArgs(allocatorAddress, amount);
		if err != nil {
			return err
		}

		var buffer bytes.Buffer
		if err := cbg.WriteByteArray(&buffer, calldata); err != nil {
			return xerrors.Errorf("failed to encode evm params as cbor: %w", err)
		}
		calldata = buffer.Bytes()

		msg := &types.Message{
			To:     fContractAddress,
			From:   from,
			Method: builtintypes.MethodsEVM.InvokeContract,
			Params: calldata,
		}

		afmt.Println("sending message...")
		smsg, err := api.MpoolPushMessage(ctx, msg, nil)
		if err != nil {
			return err
		}
		afmt.Println("waiting for message to execute...")
		wait, err := api.StateWaitMsg(ctx, smsg.Cid(), 0)
		if err != nil {
			return err
		}

		if wait.Receipt.ExitCode != 0 {
			return xerrors.Errorf("actor execution failed")
		}

		afmt.Println("OK")

		return nil
	},
}

var FilplusSetAllowanceCmd = &cli.Command{
	Name:      "set-allowance",
	Usage:     "set allowance of given allocator - can be used to remove it",
	ArgsUsage: "allocatorContractAddress allocatorAddress amount",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "from",
			Usage:    "optionally specify your address to send the message from",
		},
	},
	Action: func(cctx *cli.Context) error {
		afmt := lcli.NewAppFmt(cctx.App)

		if cctx.NArg() != 3 {
			return lcli.IncorrectNumArgs(cctx)
		}

		api, closer, err := lcli.GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := lcli.ReqContext(cctx)

		var from address.Address
		if froms := cctx.String("from"); froms == "" {
			from, err = api.WalletDefaultAddress(ctx)
		} else {
			from, err = address.NewFromString(froms)
		}
		if err != nil {
			return err
		}

		contractAddress, err := ethtypes.ParseEthAddress(cctx.Args().Get(0))
		if err != nil {
			return err
		}
		fContractAddress, err := contractAddress.ToFilecoinAddress()
		if err != nil {
			return err
		}

		allocatorAddress, err := ethtypes.ParseEthAddress(cctx.Args().Get(1))
		if err != nil {
			return err
		}

		amount := new(big.Int)
		_, err = fmt.Sscan(cctx.Args().Get(2), amount)
		if err != nil {
			return err
		}

		setAllowance := ethabi.MustParseMethod("setAllowance(address,uint256)");
		calldata := setAllowance.MustEncodeArgs(allocatorAddress, amount);
		if err != nil {
			return err
		}

		var buffer bytes.Buffer
		if err := cbg.WriteByteArray(&buffer, calldata); err != nil {
			return xerrors.Errorf("failed to encode evm params as cbor: %w", err)
		}
		calldata = buffer.Bytes()

		msg := &types.Message{
			To:     fContractAddress,
			From:   from,
			Method: builtintypes.MethodsEVM.InvokeContract,
			Params: calldata,
		}

		afmt.Println("sending message...")
		smsg, err := api.MpoolPushMessage(ctx, msg, nil)
		if err != nil {
			return err
		}
		afmt.Println("waiting for message to execute...")
		wait, err := api.StateWaitMsg(ctx, smsg.Cid(), 0)
		if err != nil {
			return err
		}

		if wait.Receipt.ExitCode != 0 {
			return xerrors.Errorf("actor execution failed")
		}

		afmt.Println("OK")

		return nil
	},
}

var FilplusAddVerifiedClientCmd = &cli.Command{
	Name:      "add-verified-client",
	Usage:     "add verified client with datacap",
	ArgsUsage: "allocatorContractAddress clientAddress amount",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "from",
			Usage:    "optionally specify your address to send the message from",
		},
	},
	Action: func(cctx *cli.Context) error {
		afmt := lcli.NewAppFmt(cctx.App)

		if cctx.NArg() != 3 {
			return lcli.IncorrectNumArgs(cctx)
		}

		api, closer, err := lcli.GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := lcli.ReqContext(cctx)

		var from address.Address
		if froms := cctx.String("from"); froms == "" {
			from, err = api.WalletDefaultAddress(ctx)
		} else {
			from, err = address.NewFromString(froms)
		}
		if err != nil {
			return err
		}

		contractAddress, err := ethtypes.ParseEthAddress(cctx.Args().Get(0))
		if err != nil {
			return err
		}
		fContractAddress, err := contractAddress.ToFilecoinAddress()
		if err != nil {
			return err
		}

		clientAddress, err := ethtypes.ParseEthAddress(cctx.Args().Get(1))
		if err != nil {
			return err
		}

		amount := new(big.Int)
		_, err = fmt.Sscan(cctx.Args().Get(2), amount)
		if err != nil {
			return err
		}

		addVerifiedClient := ethabi.MustParseMethod("addVerifiedClient(address,uint256)");
		calldata := addVerifiedClient.MustEncodeArgs(clientAddress, amount);
		if err != nil {
			return err
		}

		var buffer bytes.Buffer
		if err := cbg.WriteByteArray(&buffer, calldata); err != nil {
			return xerrors.Errorf("failed to encode evm params as cbor: %w", err)
		}
		calldata = buffer.Bytes()

		msg := &types.Message{
			To:     fContractAddress,
			From:   from,
			Method: builtintypes.MethodsEVM.InvokeContract,
			Params: calldata,
		}

		afmt.Println("sending message...")
		smsg, err := api.MpoolPushMessage(ctx, msg, nil)
		if err != nil {
			return err
		}
		afmt.Println("waiting for message to execute...")
		wait, err := api.StateWaitMsg(ctx, smsg.Cid(), 0)
		if err != nil {
			return err
		}

		if wait.Receipt.ExitCode != 0 {
			return xerrors.Errorf("actor execution failed")
		}

		afmt.Println("OK")

		return nil
	},
}
