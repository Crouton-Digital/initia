package app

import (
	"encoding/json"

	icacontrollertypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/types"
	icagenesistypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/genesis/types"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	l2slinky "github.com/initia-labs/OPinit/x/opchild/l2slinky"
	customdistrtypes "github.com/initia-labs/initia/x/distribution/types"
	customgovtypes "github.com/initia-labs/initia/x/gov/types"
	movetypes "github.com/initia-labs/initia/x/move/types"
	stakingtypes "github.com/initia-labs/initia/x/mstaking/types"
	rewardtypes "github.com/initia-labs/initia/x/reward/types"

	auctiontypes "github.com/skip-mev/block-sdk/v2/x/auction/types"
	slinkytypes "github.com/skip-mev/slinky/pkg/types"
	oracletypes "github.com/skip-mev/slinky/x/oracle/types"
)

// GenesisState - The genesis state of the blockchain is represented here as a map of raw json
// messages key'd by a identifier string.
// The identifier is used to determine which module genesis information belongs
// to so it may be appropriately routed during init chain.
// Within this application default genesis information is retrieved from
// the ModuleBasicManager which populates json from each BasicModule
// object provided to it during init.
type GenesisState map[string]json.RawMessage

// NewDefaultGenesisState generates the default state for the application.
func NewDefaultGenesisState(cdc codec.JSONCodec, bondDenom string) GenesisState {
	return GenesisState(BasicManager().DefaultGenesis(cdc)).
		ConfigureBondDenom(cdc, bondDenom).
		ConfigureICA(cdc).
		AddTimestampCurrencyPair(cdc)
}

// ConfigureBondDenom generates the default state for the application.
func (genState GenesisState) ConfigureBondDenom(cdc codec.JSONCodec, bondDenom string) GenesisState {
	// customize bond denom
	var stakingGenState stakingtypes.GenesisState
	cdc.MustUnmarshalJSON(genState[stakingtypes.ModuleName], &stakingGenState)
	stakingGenState.Params.BondDenoms = []string{bondDenom}
	genState[stakingtypes.ModuleName] = cdc.MustMarshalJSON(&stakingGenState)

	var distrGenState customdistrtypes.GenesisState
	cdc.MustUnmarshalJSON(genState[distrtypes.ModuleName], &distrGenState)
	distrGenState.Params.RewardWeights = []customdistrtypes.RewardWeight{{Denom: bondDenom, Weight: math.LegacyOneDec()}}
	genState[distrtypes.ModuleName] = cdc.MustMarshalJSON(&distrGenState)

	var crisisGenState crisistypes.GenesisState
	cdc.MustUnmarshalJSON(genState[crisistypes.ModuleName], &crisisGenState)
	crisisGenState.ConstantFee.Denom = bondDenom
	genState[crisistypes.ModuleName] = cdc.MustMarshalJSON(&crisisGenState)

	var govGenState customgovtypes.GenesisState
	cdc.MustUnmarshalJSON(genState[govtypes.ModuleName], &govGenState)
	govGenState.Params.MinDeposit[0].Denom = bondDenom
	govGenState.Params.ExpeditedMinDeposit[0].Denom = bondDenom
	govGenState.Params.EmergencyMinDeposit[0].Denom = bondDenom
	genState[govtypes.ModuleName] = cdc.MustMarshalJSON(&govGenState)

	var rewardGenState rewardtypes.GenesisState
	cdc.MustUnmarshalJSON(genState[rewardtypes.ModuleName], &rewardGenState)
	rewardGenState.Params.RewardDenom = bondDenom
	genState[rewardtypes.ModuleName] = cdc.MustMarshalJSON(&rewardGenState)

	var moveGenState movetypes.GenesisState
	cdc.MustUnmarshalJSON(genState[movetypes.ModuleName], &moveGenState)
	moveGenState.Params.BaseDenom = bondDenom
	genState[movetypes.ModuleName] = cdc.MustMarshalJSON(&moveGenState)

	// Auction module genesis-state bond denom configuration
	var auctionGenState auctiontypes.GenesisState
	cdc.MustUnmarshalJSON(genState[auctiontypes.ModuleName], &auctionGenState)
	auctionGenState.Params.ReserveFee.Denom = bondDenom
	auctionGenState.Params.MinBidIncrement.Denom = bondDenom
	genState[auctiontypes.ModuleName] = cdc.MustMarshalJSON(&auctionGenState)

	return genState
}

func (genState GenesisState) AddTimestampCurrencyPair(cdc codec.JSONCodec) GenesisState {
	var oracleGenState oracletypes.GenesisState
	cdc.MustUnmarshalJSON(genState[oracletypes.ModuleName], &oracleGenState)

	cp, err := slinkytypes.CurrencyPairFromString(l2slinky.ReservedCPTimestamp)
	if err != nil {
		panic(err)
	}

	oracleGenState.CurrencyPairGenesis = append(oracleGenState.CurrencyPairGenesis, oracletypes.CurrencyPairGenesis{
		CurrencyPair:      cp,
		CurrencyPairPrice: nil,
		Nonce:             0,
	})
	oracleGenState.NextId = 1
	genState[oracletypes.ModuleName] = cdc.MustMarshalJSON(&oracleGenState)
	return genState
}

func (genState GenesisState) ConfigureICA(cdc codec.JSONCodec) GenesisState {
	// create ICS27 Controller submodule params
	controllerParams := icacontrollertypes.Params{
		ControllerEnabled: true,
	}

	// create ICS27 Host submodule params
	hostParams := icahosttypes.Params{
		HostEnabled: true,
		AllowMessages: []string{
			authzMsgExec,
			authzMsgGrant,
			authzMsgRevoke,
			bankMsgSend,
			bankMsgMultiSend,
			distrMsgSetWithdrawAddr,
			distrMsgWithdrawValidatorCommission,
			distrMsgFundCommunityPool,
			distrMsgWithdrawDelegatorReward,
			feegrantMsgGrantAllowance,
			feegrantMsgRevokeAllowance,
			govMsgVoteWeighted,
			govMsgSubmitProposal,
			govMsgDeposit,
			govMsgVote,
			groupCreateGroup,
			groupCreateGroupPolicy,
			groupExec,
			groupLeaveGroup,
			groupSubmitProposal,
			groupUpdateGroupAdmin,
			groupUpdateGroupMember,
			groupUpdateGroupPolicyAdmin,
			groupUpdateGroupPolicyDecisionPolicy,
			groupVote,
			groupWithdrawProposal,
			stakingMsgEditValidator,
			stakingMsgDelegate,
			stakingMsgUndelegate,
			stakingMsgBeginRedelegate,
			stakingMsgCreateValidator,
			transferMsgTransfer,
			nftTransferMsgTransfer,
			moveMsgPublishModuleBundle,
			moveMsgExecuteEntryFunction,
			moveMsgExecuteScript,
		},
	}

	var icaGenState icagenesistypes.GenesisState
	cdc.MustUnmarshalJSON(genState[icatypes.ModuleName], &icaGenState)
	icaGenState.ControllerGenesisState.Params = controllerParams
	icaGenState.HostGenesisState.Params = hostParams
	genState[icatypes.ModuleName] = cdc.MustMarshalJSON(&icaGenState)

	return genState
}
