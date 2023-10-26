package evidence

import (
	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/evidence/types"

	"github.com/initia-labs/initia/x/evidence/keeper"
)

// NewHandler returns a handler for evidence messages.
func NewHandler(k keeper.Keeper) sdk.Handler {
	msgServer := keeper.NewMsgServerImpl(k)

	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		ctx = ctx.WithEventManager(sdk.NewEventManager())

		switch msg := msg.(type) {
		case *types.MsgSubmitEvidence:
			res, err := msgServer.SubmitEvidence(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)

		default:
			return nil, errors.Wrapf(sdkerrors.ErrUnknownRequest, "unrecognized %s message type: %T", types.ModuleName, msg)
		}
	}
}