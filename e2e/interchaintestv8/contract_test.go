package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	sdkmath "cosmossdk.io/math"

	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"
	ibctesting "github.com/cosmos/ibc-go/v8/testing"

	"github.com/strangelove-ventures/interchaintest/v8/testutil"

	"github.com/srdtrk/go-codegen/e2esuite/v8/e2esuite"
	"github.com/srdtrk/go-codegen/e2esuite/v8/types/callbackcounter"
	"github.com/srdtrk/go-codegen/e2esuite/v8/types/cwicacontroller"
)

// ContactTestSuite is a suite of tests that wraps the TestSuite
// and can provide additional functionality
type ContractTestSuite struct {
	e2esuite.TestSuite

	// this line is used by go-codegen # suite/contract
}

// SetupSuite calls the underlying ContractTestSuite's SetupSuite method
func (s *ContractTestSuite) SetupSuite(ctx context.Context) {
	s.TestSuite.SetupSuite(ctx)
}

// TestWithContractTestSuite is the boilerplate code that allows the test suite to be run
func TestWithContractTestSuite(t *testing.T) {
	suite.Run(t, new(ContractTestSuite))
}

// TestContract is an example test function that will be run by the test suite
func (s *ContractTestSuite) TestContract() {
	ctx := context.Background()

	s.SetupSuite(ctx)

	wasmd1, wasmd2 := s.ChainA, s.ChainB

	// Add your test code here. For example, upload and instantiate a contract:
	// This boilerplate may be moved to SetupSuite if it is common to all tests.
	var (
		contract         *cwicacontroller.Contract
		callbackContract *callbackcounter.Contract
	)
	s.Require().True(s.Run("UploadAndInstantiateContracts", func() {
		// Upload the contract code to the chain.
		ccCodeID, err := wasmd1.StoreContract(ctx, s.UserA.KeyName(), "../../artifacts/callback_counter.wasm")
		s.Require().NoError(err)

		callbackContract, err = callbackcounter.Instantiate(ctx, s.UserA.KeyName(), ccCodeID, "", s.ChainA, callbackcounter.InstantiateMsg{})
		s.Require().NoError(err)
		s.Require().NotEmpty(callbackContract.Address)

		codeID, err := wasmd1.StoreContract(ctx, s.UserA.KeyName(), "../../artifacts/cw_ica_controller.wasm")
		s.Require().NoError(err)

		// Instantiate the contract using contract helpers.
		// This will an error if the instantiate message is invalid.
		channelOrder := cwicacontroller.IbcOrder_OrderOrdered
		contract, err = cwicacontroller.Instantiate(ctx, s.UserA.KeyName(), codeID, "", wasmd1, cwicacontroller.InstantiateMsg{
			Owner:           nil,
			SendCallbacksTo: &callbackContract.Address,
			ChannelOpenInitOptions: cwicacontroller.ChannelOpenInitOptions{
				ChannelOrdering:          &channelOrder,
				ConnectionId:             ibctesting.FirstConnectionID,
				CounterpartyConnectionId: ibctesting.FirstConnectionID,
			},
		}, "--gas", "500000")
		s.Require().NoError(err)
		s.Require().NotEmpty(contract.Address)

		// Wait for the chains to process the channel handshake.
		s.Require().NoError(testutil.WaitForBlocks(ctx, 5, wasmd1, wasmd2))
	}))

	var icaAddress string
	s.Require().True(s.Run("Verify_ChannelOpenHandshake", func() {
		// Check that the channel was opened.
		channel, err := contract.QueryClient().GetChannel(ctx, &cwicacontroller.QueryMsg_GetChannel{})
		s.Require().NoError(err)

		s.Require().Equal(cwicacontroller.Status_StateOpen, channel.ChannelStatus)
		s.Require().Equal(cwicacontroller.IbcOrder_OrderOrdered, channel.Channel.Order)
		s.Require().Equal(ibctesting.FirstChannelID, channel.Channel.Endpoint.ChannelId)
		s.Require().Equal(contract.Port(), channel.Channel.Endpoint.PortId)
		s.Require().Equal(ibctesting.FirstConnectionID, channel.Channel.ConnectionId)
		s.Require().Equal(ibctesting.FirstChannelID, channel.Channel.CounterpartyEndpoint.ChannelId)
		s.Require().Equal(icatypes.HostPortID, channel.Channel.CounterpartyEndpoint.PortId)

		// Check that the ica address is set
		contractState, err := contract.QueryClient().GetContractState(ctx, &cwicacontroller.QueryMsg_GetContractState{})
		s.Require().NoError(err)

		s.Require().NotEmpty(contractState.IcaInfo)
		s.Require().NotEmpty(contractState.IcaInfo.IcaAddress)

		icaAddress = contractState.IcaInfo.IcaAddress
	}))

	s.Require().True(s.Run("Test_CosmosMsg_Delegate", func() {
		// Fund the ICA account
		s.FundAddressChainB(ctx, icaAddress)

		// Check the initial balance of the ICA account
		intialBalance, err := wasmd2.GetBalance(ctx, icaAddress, wasmd2.Config().Denom)
		s.Require().NoError(err)

		validator, err := wasmd2.Validators[0].KeyBech32(ctx, "validator", "val")
		s.Require().NoError(err)

		// Stake some tokens through CosmosMsgs:
		stakeAmount := sdkmath.NewInt(10_000_000)
		stakeCosmosMsg := cwicacontroller.CosmosMsg_for_Empty{
			Staking: &cwicacontroller.CosmosMsg_for_Empty_Staking{
				Delegate: &cwicacontroller.StakingMsg_Delegate{
					Validator: validator,
					Amount: cwicacontroller.Coin{
						Denom:  wasmd2.Config().Denom,
						Amount: cwicacontroller.Uint128(stakeAmount.String()),
					},
				},
			},
		}

		execMsg := cwicacontroller.ExecuteMsg{
			SendCosmosMsgs: &cwicacontroller.ExecuteMsg_SendCosmosMsgs{
				Messages: []cwicacontroller.CosmosMsg_for_Empty{stakeCosmosMsg},
			},
		}

		_, err = contract.Execute(ctx, s.UserA.KeyName(), execMsg)
		s.Require().NoError(err)

		// Wait for the chains to process the transaction.
		s.Require().NoError(testutil.WaitForBlocks(ctx, 5, wasmd1, wasmd2))

		// Check the final balance of the ICA account
		finalBalance, err := wasmd2.GetBalance(ctx, icaAddress, wasmd2.Config().Denom)
		s.Require().NoError(err)
		s.Require().Equal(intialBalance.Sub(stakeAmount), finalBalance)

		// Check the final delegations
		delResp, err := e2esuite.GRPCQuery[stakingtypes.QueryDelegationResponse](ctx, wasmd2, &stakingtypes.QueryDelegationRequest{
			DelegatorAddr: icaAddress,
			ValidatorAddr: validator,
		})
		s.Require().NoError(err)
		s.Require().Equal(stakeAmount, delResp.DelegationResponse.Balance.Amount)

		// Verify that the callbacks were called
		callbacks, err := callbackContract.QueryClient().GetCallbackCounter(ctx, &callbackcounter.QueryMsg_GetCallbackCounter{})
		s.Require().NoError(err)
		s.Require().Equal(int(1), callbacks.Success)
		s.Require().Equal(int(0), callbacks.Error)
		s.Require().Equal(int(0), callbacks.Timeout)
	}))
}
