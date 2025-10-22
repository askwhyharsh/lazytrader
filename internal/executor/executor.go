// internal/executor/executor.go
package executor

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	// "github.com/ethereum/go-ethereum/common"
	// "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	
	"github.com/askwhyharsh/lazytrader/internal/config"
	"github.com/askwhyharsh/lazytrader/internal/database"
)

type Executor struct {
	cfg         *config.Config
	db          *database.DB
	client      *ethclient.Client
	privateKey  *ecdsa.PrivateKey
	chainID     *big.Int
}

type TradeRequest struct {
	MarketID  string
	TokenID   string
	Outcome   string
	Side      string  // "buy" or "sell"
	Amount    float64
	Price     float64
}

func New(cfg *config.Config, db *database.DB) *Executor {
	return &Executor{
		cfg: cfg,
		db:  db,
	}
}

func (e *Executor) Start(ctx context.Context) error {
	log.Println("Starting execution engine...")

	// Connect to Polygon RPC
	client, err := ethclient.Dial(e.cfg.PolygonRPCURL)
	if err != nil {
		return fmt.Errorf("failed to connect to Polygon: %w", err)
	}
	e.client = client

	// // Load admin private key
	// privateKey, err := crypto.HexToECDSA(e.cfg.AdminPrivateKey)
	// if err != nil {
	// 	return fmt.Errorf("invalid private key: %w", err)
	// }
	// e.privateKey = privateKey
	// e.chainID = big.NewInt(e.cfg.ChainID)

	// log.Printf("Executor ready with address: %s", crypto.PubkeyToAddress(privateKey.PublicKey).Hex())

	// In production, listen for trade signals from ingestion layer
	// For now, just keep the executor alive
	<-ctx.Done()
	return nil
}

func (e *Executor) ExecuteTrade(req TradeRequest) error {
	log.Printf("Executing trade: %s %s %.2f @ %.4f", req.Side, req.MarketID, req.Amount, req.Price)

	// Create position record
	position, err := e.db.CreatePosition(req.MarketID, req.TokenID, req.Outcome, req.Amount, req.Price)
	if err != nil {
		return fmt.Errorf("failed to create position: %w", err)
	}

	// Create trade record
	trade, err := e.db.CreateTrade(position.ID, "", req.Side, req.Amount, req.Price)
	if err != nil {
		return fmt.Errorf("failed to create trade: %w", err)
	}

	// Execute on-chain trade
	txHash, err := e.submitTrade(req)
	if err != nil {
		e.db.UpdateTradeStatus(trade.ID, "failed", "")
		return fmt.Errorf("failed to submit trade: %w", err)
	}

	// Update trade with tx hash
	if err := e.db.UpdateTradeStatus(trade.ID, "confirmed", txHash); err != nil {
		log.Printf("Failed to update trade status: %v", err)
	}

	log.Printf("Trade executed: %s", txHash)
	return nil
}

func (e *Executor) submitTrade(req TradeRequest) (string, error) {
	// Build transaction to vault contract
	auth, err := bind.NewKeyedTransactorWithChainID(e.privateKey, e.chainID)
	if err != nil {
		return "", err
	}

	// Gas settings
	auth.GasLimit = uint64(300000)
	
	// In production, call the vault contract's executeTrade function
	// For now, return a mock tx hash
	mockTxHash := fmt.Sprintf("0x%064x", 12345)
	
	log.Printf("Submitted transaction: %s", mockTxHash)
	return mockTxHash, nil
}

// func (e *Executor) GetVaultBalance() (*big.Int, error) {
// 	if e.client == nil {
// 		return big.NewInt(0), fmt.Errorf("client not initialized")
// 	}

// 	// vaultAddr := common.HexToAddress(e.cfg.VaultContractAddr)
// 	// balance, err := e.client.BalanceAt(context.Background(), vaultAddr, nil)
// 	// if err != nil {
// 	// 	return nil, err
// 	// }
// 	bal := 1000.(big.Int)
// 	// return balance, nil
// 	return &bal, nil
// }

func (e *Executor) CalculateTotalShares() (float64, error) {
	// Query all users and sum shares
	// For now, return mock value
	return 1000.0, nil
}

func (e *Executor) CalculateVaultValue() (float64, error) {
	// Calculate total vault value based on:
	// - USDC balance
	// - Open positions (marked to market)
	// For now, return mock value
	return 10000.0, nil
}