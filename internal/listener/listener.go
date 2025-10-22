// internal/listener/polymarket_listener.go
package listener

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	
	"github.com/askwhyharsh/lazytrader/internal/config"
	"github.com/askwhyharsh/lazytrader/internal/database"
)

// Polymarket contract addresses on Polygon
const (
	CTF_EXCHANGE_ADDR      = "0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E"
	NEG_RISK_EXCHANGE_ADDR = "0xC5d563A36AE78145C45a50134d48A1215220f80a"
	CTF_ADDR               = "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"
	USDC_ADDR              = "0x2791bca1f2de4661ed88a30c99a7a9449aa84174" // USDC.e on Polygon
)

type PolymarketListener struct {
	cfg       *config.Config
	db        *database.DB
	client    *ethclient.Client
	
	// Contract ABIs
	exchangeABI abi.ABI
	
	// Event signatures
	orderFilledSig common.Hash
	ordersMatchedSig common.Hash
	
	// Tracked traders
	topTraders map[string]bool
}

// OrderFilledEvent represents the OrderFilled event from CTF Exchange
type OrderFilledEvent struct {
    OrderHash          [32]byte
    Maker              common.Address
    Taker              common.Address
    MakerAssetId       *big.Int
    TakerAssetId       *big.Int
    MakerAmountFilled  *big.Int
    TakerAmountFilled  *big.Int
    Fee                *big.Int
}


// OrdersMatchedEvent represents batch order matching
type OrdersMatchedEvent struct {
	TakerOrderHash [32]byte
	TakerOrderMaker common.Address
	MakerAssetId   *big.Int
	TakerAssetId   *big.Int
	MakerAmountFilled *big.Int
	TakerAmountFilled *big.Int
}

func NewPolymarketListener(cfg *config.Config, db *database.DB) (*PolymarketListener, error) {
	client, err := ethclient.Dial(cfg.PolygonRPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Polygon: %w", err)
	}
	
	// Parse the exchange ABI
	exchangeABI, err := abi.JSON(strings.NewReader(CTFExchangeABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}
	
	// Calculate event signatures
	orderFilledSig := crypto.Keccak256Hash([]byte("OrderFilled(bytes32,address,address,uint256,uint256,uint256,uint256,uint256)"))
	ordersMatchedSig := crypto.Keccak256Hash([]byte("OrdersMatched(bytes32,bytes32[],uint256,uint256,uint256,uint256)"))
	
	return &PolymarketListener{
		cfg:              cfg,
		db:               db,
		client:           client,
		exchangeABI:      exchangeABI,
		orderFilledSig:   orderFilledSig,
		ordersMatchedSig: ordersMatchedSig,
		topTraders:       make(map[string]bool),
	}, nil
}

func (l *PolymarketListener) Start(ctx context.Context) error {
	log.Println("Starting Polymarket event listener...")
	
	// Update top traders list periodically
	go l.updateTopTraders(ctx)
	
	// Subscribe to new blocks
	headers := make(chan *types.Header)
	sub, err := l.client.SubscribeNewHead(ctx, headers)
	if err != nil {
		return fmt.Errorf("failed to subscribe to new heads: %w", err)
	}
	defer sub.Unsubscribe()
	
	// Also poll old blocks in case we missed any
	go l.pollHistoricalBlocks(ctx)
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-sub.Err():
			log.Printf("Subscription error: %v", err)
			return err
		case header := <-headers:
			if err := l.processBlock(ctx, header.Number); err != nil {
				log.Printf("Error processing block %d: %v", header.Number.Uint64(), err)
			}
		}
	}
}

func (l *PolymarketListener) updateTopTraders(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			traders, err := l.db.GetTopTraders(l.cfg.TopTradersCount)
			if err != nil {
				log.Printf("Failed to get top traders: %v", err)
				continue
			}
			
			// Update map
			l.topTraders = make(map[string]bool)
			for _, trader := range traders {
				l.topTraders[strings.ToLower(trader)] = true
			}
			
			log.Printf("Updated top traders list: %d traders", len(l.topTraders))
		}
	}
}

func (l *PolymarketListener) processBlock(ctx context.Context, blockNumber *big.Int) error {
	// Query for OrderFilled events from both exchanges
	query := ethereum.FilterQuery{
		FromBlock: blockNumber,
		ToBlock:   blockNumber,
		Addresses: []common.Address{
			common.HexToAddress(CTF_EXCHANGE_ADDR),
			common.HexToAddress(NEG_RISK_EXCHANGE_ADDR),
		},
		Topics: [][]common.Hash{
			{l.orderFilledSig, l.ordersMatchedSig},
		},
	}
	
	logs, err := l.client.FilterLogs(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to filter logs: %w", err)
	}
	
	for _, vLog := range logs {
		if err := l.processLog(vLog); err != nil {
			log.Printf("Error processing log: %v", err)
			// stop loop
			break
		}
	}
	
	return nil
}

func (l *PolymarketListener) processLog(vLog types.Log) error {
	fmt.Println(vLog.Topics)
	// Check if this is an OrderFilled event
	if vLog.Topics[0] == l.orderFilledSig {
		return l.processOrderFilled(vLog)
	}
	
	// Check if this is an OrdersMatched event
	if vLog.Topics[0] == l.ordersMatchedSig {
		return l.processOrdersMatched(vLog)
	}
	
	return nil
}

func (l *PolymarketListener) processOrderFilled(vLog types.Log) error {
	// Parse the event
	event := &OrderFilledEvent{}
	err := l.exchangeABI.UnpackIntoInterface(event, "OrderFilled", vLog.Data)
	if err != nil {
		return fmt.Errorf("failed to unpack OrderFilled: %w", err)
	}
	
	// Extract indexed parameters from topics
	if len(vLog.Topics) >= 2 {
		event.OrderHash = [32]byte(vLog.Topics[1])
	}
	
	maker := event.Maker.Hex()
	taker := event.Taker.Hex()
	
	// Check if maker or taker is a top trader we're tracking
	makerIsTop := l.topTraders[strings.ToLower(maker)]
	takerIsTop := l.topTraders[strings.ToLower(taker)]
	
	if !makerIsTop && !takerIsTop {
		log.Printf(" Not a top trader activity :(")
		return nil // Skip if not from top trader
	}
	
	log.Printf("🔔 Top trader activity detected!")
	log.Printf("   Maker: %s (Top: %v)", maker[:10], makerIsTop)
	log.Printf("   Taker: %s (Top: %v)", taker[:10], takerIsTop)
	log.Printf("   Maker Asset: %s", event.MakerAssetId.String())
	log.Printf("   Taker Asset: %s", event.TakerAssetId.String())
	log.Printf("   Maker Amount: %s", event.MakerAmountFilled.String())
	log.Printf("   Taker Amount: %s", event.TakerAmountFilled.String())
	log.Printf("   Tx: %s", vLog.TxHash.Hex())
	
	// Determine who initiated (maker or taker) and what they're doing
	tradeSignal := l.extractTradeSignal(event, makerIsTop, takerIsTop)
	// Store in database for executor to pick up
	return l.storeTradeSignal(tradeSignal, vLog.TxHash.Hex())
}

func (l *PolymarketListener) processOrdersMatched(vLog types.Log) error {
	// Similar to OrderFilled but for batch matching
	log.Printf("OrdersMatched event in tx: %s", vLog.TxHash.Hex())
	return nil
}

type TradeSignal struct {
	Trader      string
	Side        string // "BUY" or "SELL"
	MarketID    string
	TokenID     *big.Int
	Amount      *big.Int
	Price       *big.Int
	TxHash      string
}

func (l *PolymarketListener) extractTradeSignal(event *OrderFilledEvent, makerIsTop, takerIsTop bool) *TradeSignal {
	signal := &TradeSignal{}
	
	// If maker asset is 0, maker is buying (providing USDC) // so we can buy - if maker is top trader
	// If taker asset is 0, taker is buying (providing USDC) // 
	
	if makerIsTop {
		signal.Trader = event.Maker.Hex()
		if event.MakerAssetId.Cmp(big.NewInt(0)) == 0 {
			signal.Side = "BUY"
			signal.TokenID = event.TakerAssetId
			signal.Amount = event.TakerAmountFilled
		} else {
			signal.Side = "SELL"
			signal.TokenID = event.MakerAssetId
			signal.Amount = event.MakerAmountFilled
		}
	} else if takerIsTop {
		signal.Trader = event.Taker.Hex()
		if event.TakerAssetId.Cmp(big.NewInt(0)) == 0 {
			signal.Side = "BUY"
			signal.TokenID = event.MakerAssetId
			signal.Amount = event.MakerAmountFilled
		} else {
			signal.Side = "SELL"
			signal.TokenID = event.TakerAssetId
			signal.Amount = event.TakerAmountFilled
		}
	}
	
	// Calculate price (simplified)
	if signal.Side == "BUY" && event.MakerAmountFilled.Cmp(big.NewInt(0)) > 0 {
		signal.Price = new(big.Int).Div(
			new(big.Int).Mul(event.TakerAmountFilled, big.NewInt(1e6)),
			event.MakerAmountFilled,
		)
	}
	fmt.Printf("+v%s", signal)
	return signal
}

func (l *PolymarketListener) storeTradeSignal(signal *TradeSignal, txHash string) error {
	// Store in database - executor will pick this up
	// For now, just log
	log.Printf("📝 Storing trade signal: %s %s token %s amount %s",
		signal.Trader[:10], signal.Side, signal.TokenID.String(), signal.Amount.String())
	return nil
}

func (l *PolymarketListener) pollHistoricalBlocks(ctx context.Context) {
	// Poll for any missed blocks periodically
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Implement backfill logic if needed
		}
	}
}

// Minimal CTF Exchange ABI (just the events we need)
const CTFExchangeABI = `[
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "orderHash", "type": "bytes32"},
			{"indexed": true, "name": "maker", "type": "address"},
			{"indexed": true, "name": "taker", "type": "address"},
			{"indexed": false, "name": "makerAssetId", "type": "uint256"},
			{"indexed": false, "name": "takerAssetId", "type": "uint256"},
			{"indexed": false, "name": "makerAmountFilled", "type": "uint256"},
			{"indexed": false, "name": "takerAmountFilled", "type": "uint256"},
			{"indexed": false, "name": "fee", "type": "uint256"}
		],
		"name": "OrderFilled",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "takerOrderHash", "type": "bytes32"},
			{"indexed": false, "name": "takerOrderMaker", "type": "address"},
			{"indexed": false, "name": "makerAssetId", "type": "uint256"},
			{"indexed": false, "name": "takerAssetId", "type": "uint256"},
			{"indexed": false, "name": "makerAmountFilled", "type": "uint256"},
			{"indexed": false, "name": "takerAmountFilled", "type": "uint256"}
		],
		"name": "OrdersMatched",
		"type": "event"
	}
]`