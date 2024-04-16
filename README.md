# contract-allocator-cli

## Quick start

```
make
./contract-allocator-cli --help
```

You can also install globally in `/usr/local/bin/`:
```
make && make install
contract-allocator-cli --help
```

## Commands

### `deploy-allocator-contract`

Deploy new instance of allocator contract.

```
USAGE:
    contract-allocator-cli deploy-allocator-contract [command options] registryAddress initialContractOwner

OPTIONS:
   --from value  optionally specify your address to send the message from
```

* `registryAddress` - EVM address of the Allocator Contract Registry
    * testnet & mainnet addresses TBD
* `initialContractOwner` - EVM address of the contract owner. Contract owner will be able to manage allowance using `add-allowance` and `set-allowance` commands.

### `list-contracts`

List registered allocator contracts.

```
USAGE:
   contract-allocator-cli list-contracts [command options] registryAddress
```

* `registryAddress` - EVM address of the Allocator Contract Registry
    * testnet & mainnet addresses TBD

### `list-allocators`

List allocators in given allocator contract

```
USAGE:
   contract-allocator-cli list-allocators [command options] allocatorContractAddress
```

* `allocatorContractAddress` - EVM address of the Allocator Contract

### `add-allowance`

Grant allowance to an allocator.

```
USAGE:
   contract-allocator-cli add-allowance [command options] allocatorContractAddress allocatorAddress amount

OPTIONS:
   --from value  optionally specify your address to send the message from
```

* `allocatorContractAddress` - EVM address of the Allocator Contract
* `allocatorAddress` - EVM address of the Allocator to grant allowance to
* `amount` - amount of allowance to grant

### `set-allowance`

Set the allowance of a given allocator - can be used to remove it.

```
USAGE:
   contract-allocator-cli set-allowance [command options] allocatorContractAddress allocatorAddress amount

OPTIONS:
   --from value  optionally specify your address to send the message from
```

* `allocatorContractAddress` - EVM address of the Allocator Contract
* `allocatorAddress` - EVM address of the Allocator
* `amount` - the new allowance

### `add-verified-client`

Add verified client with datacap.

```
USAGE:
   contract-allocator-cli add-verified-client [command options] allocatorContractAddress clientAddress amount

OPTIONS:
   --from value  optionally specify your address to send the message from
```

* `allocatorContractAddress` - EVM address of the Allocator Contract
* `clientAddress` - EVM address of the client
* `amount` - datacap to grant