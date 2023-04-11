package relayer_test

import (
	"context"
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/scroll-tech/go-ethereum/accounts/abi/bind"
	"github.com/scroll-tech/go-ethereum/common"
	etypes "github.com/scroll-tech/go-ethereum/core/types"
	"github.com/scroll-tech/go-ethereum/ethclient"
	"github.com/scroll-tech/go-ethereum/log"
	"github.com/stretchr/testify/assert"

	"scroll-tech/common/docker"
	"scroll-tech/common/types"

	"scroll-tech/bridge/config"
)

var (
	// config
	cfg *config.Config

	base *docker.App

	l1ChainID *big.Int
	l2ChainID *big.Int

	// l1geth client
	l1Cli *ethclient.Client
	// l2geth client
	l2Cli *ethclient.Client

	// block trace
	wrappedBlock1 *types.WrappedBlock
	wrappedBlock2 *types.WrappedBlock

	// batch data
	batchData1 *types.BatchData
	batchData2 *types.BatchData
)

func setupEnv(t *testing.T) (err error) {
	// Load config.
	cfg, err = config.NewConfig("../config.json")
	assert.NoError(t, err)

	base.RunImages(t)

	cfg.L2Config.RelayerConfig.SenderConfig.Endpoint = base.L1GethEndpoint()
	cfg.L1Config.RelayerConfig.SenderConfig.Endpoint = base.L2GethEndpoint()
	cfg.DBConfig.DSN = base.DBEndpoint()

	// Create l1geth client.
	l1Cli, err = base.L1Client()
	assert.NoError(t, err)
	// Create l2geth client.
	l2Cli, err = base.L2Client()
	assert.NoError(t, err)

	l1ChainID, err = l1Cli.ChainID(context.Background())
	assert.NoError(t, err)
	l2ChainID, err = l2Cli.ChainID(context.Background())
	assert.NoError(t, err)

	templateBlockTrace1, err := os.ReadFile("../../common/testdata/blockTrace_02.json")
	if err != nil {
		return err
	}
	// unmarshal blockTrace
	wrappedBlock1 = &types.WrappedBlock{}
	if err = json.Unmarshal(templateBlockTrace1, wrappedBlock1); err != nil {
		return err
	}
	parentBatch1 := &types.BlockBatch{
		Index:     0,
		Hash:      "0x0cc6b102c2924402c14b2e3a19baccc316252bfdc44d9ec62e942d34e39ec729",
		StateRoot: "0x2579122e8f9ec1e862e7d415cef2fb495d7698a8e5f0dddc5651ba4236336e7d",
	}
	batchData1 = types.NewBatchData(parentBatch1, []*types.WrappedBlock{wrappedBlock1}, nil)

	templateBlockTrace2, err := os.ReadFile("../../common/testdata/blockTrace_03.json")
	if err != nil {
		return err
	}
	// unmarshal blockTrace
	wrappedBlock2 = &types.WrappedBlock{}
	if err = json.Unmarshal(templateBlockTrace2, wrappedBlock2); err != nil {
		return err
	}
	parentBatch2 := &types.BlockBatch{
		Index:     batchData1.Batch.BatchIndex,
		Hash:      batchData1.Hash().Hex(),
		StateRoot: batchData1.Batch.NewStateRoot.String(),
	}
	batchData2 = types.NewBatchData(parentBatch2, []*types.WrappedBlock{wrappedBlock2}, nil)

	log.Info("batchHash", "batchhash1", batchData1.Hash().Hex(), "batchhash2", batchData2.Hash().Hex())

	return err
}

func TestMain(m *testing.M) {
	base = docker.NewDockerApp()

	m.Run()

	base.Free()
}

func TestFunctions(t *testing.T) {
	if err := setupEnv(t); err != nil {
		t.Fatal(err)
	}
	// Run l1 relayer test cases.
	t.Run("TestCreateNewL1Relayer", testCreateNewL1Relayer)
	t.Run("testL1CheckSubmittedMessages", testL1CheckSubmittedMessages)
	// Run l2 relayer test cases.
	t.Run("TestCreateNewRelayer", testCreateNewRelayer)
	t.Run("TestL2RelayerProcessSaveEvents", testL2RelayerProcessSaveEvents)
	t.Run("TestL2RelayerProcessCommittedBatches", testL2RelayerProcessCommittedBatches)
	t.Run("TestL2RelayerSkipBatches", testL2RelayerSkipBatches)
	t.Run("testL2CheckSubmittedMessages", testL2CheckSubmittedMessages)
	t.Run("testL2CheckRollupCommittingBatches", testL2CheckRollupCommittingBatches)
	t.Run("testL2CheckRollupFinalizingBatches", testL2CheckRollupFinalizingBatches)
}

func mockTx(auth *bind.TransactOpts, cli *ethclient.Client) (*etypes.Transaction, error) {
	if auth.Nonce == nil {
		auth.Nonce = big.NewInt(0)
	} else {
		auth.Nonce.Add(auth.Nonce, big.NewInt(1))
	}
	nonce, err := cli.PendingNonceAt(context.Background(), auth.From)
	if err != nil {
		return nil, err
	}
	auth.Nonce = big.NewInt(0).SetUint64(nonce)

	tx := etypes.NewTx(&etypes.LegacyTx{
		Nonce:    auth.Nonce.Uint64(),
		To:       &auth.From,
		Value:    big.NewInt(0),
		Gas:      500000,
		GasPrice: big.NewInt(500000),
		Data:     common.Hex2Bytes("1212121212121212121212121212121212121212121212121212121212121212"),
	})

	return auth.Signer(auth.From, tx)
}
