// internal/ingestion/ingestion.go
package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/askwhyharsh/lazytrader/internal/config"
	"github.com/askwhyharsh/lazytrader/internal/database"
)

const (
	POLYMARKET_LEADERBOARD_API = "https://data-api.polymarket.com/v1/leaderboard"
)
type Ingestion struct {
	cfg            *config.Config
	db             *database.DB
	client         *http.Client
	lastCheckTime  map[string]int64 // Track last check time per trader
}

type LeaderboardEntry struct {
	Address string  `json:"address"`
	PnL     float64 `json:"pnl"`
	WinRate float64 `json:"win_rate"`
}

// Polymarket API response structure
type PolymarketLeaderboardEntry struct {
	Rank         string  `json:"rank"`
	ProxyWallet  string  `json:"proxyWallet"`
	UserName     string  `json:"userName"`
	Vol          float64 `json:"vol"`
	PnL          float64 `json:"pnl"`
	ProfileImage string  `json:"profileImage"`
}

func New(cfg *config.Config, db *database.DB) *Ingestion {
	return &Ingestion{
		cfg:           cfg,
		db:            db,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		lastCheckTime: make(map[string]int64),
	}
}

func (i *Ingestion) Start(ctx context.Context) error {
	log.Println("Starting ingestion service with Polymarket Data API...")

	// Update top traders leaderboard from Polymarket API
	leaderboardTicker := time.NewTicker(10 * time.Minute)
	defer leaderboardTicker.Stop()

	// Poll for new trades from top traders (via event listener)
	// The event listener will handle the actual trade detection

	// Initial leaderboard update
	if err := i.updateLeaderboardFromAPI(ctx); err != nil {
		log.Printf("Failed initial leaderboard update: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-leaderboardTicker.C:
			if err := i.updateLeaderboardFromAPI(ctx); err != nil {
				log.Printf("Failed to update leaderboard: %v", err)
			}
		}
	}
}

// updateLeaderboardFromAPI fetches top traders from Polymarket Data API
func (i *Ingestion) updateLeaderboardFromAPI(ctx context.Context) error {
	log.Println("🔍 Fetching top traders from Polymarket Data API...")

	// Build API URL with parameters
	// timePeriod: "day", "week", "month"
	// orderBy: "VOL" (volume) or "PNL" (profit/loss)
	// category: "overall"
	timePeriod := "week"
	orderBy := "PNL" // Order by profit for best traders
	limit := 20
	offset := 0

	url := fmt.Sprintf("%s?timePeriod=%s&orderBy=%s&limit=%d&offset=%d&category=overall",
		POLYMARKET_LEADERBOARD_API, timePeriod, orderBy, limit, offset)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := i.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch from Polymarket API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, body)
	}

	var entries []PolymarketLeaderboardEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(entries) == 0 {
		log.Println("⚠️  No leaderboard entries returned from API")
		return nil
	}

	// Store top traders in database
	count := 0
	for _, entry := range entries {
		// Filter by minimum profit threshold
		if entry.PnL >= i.cfg.MinProfitThreshold {
			// Calculate approximate win rate (we don't have exact data from this API)
			// For now, assume higher PnL = higher win rate
			estimatedWinRate := 0.5 + (entry.PnL / (entry.Vol + 1)) * 0.3
			if estimatedWinRate > 0.9 {
				estimatedWinRate = 0.9
			}

			if err := i.db.UpsertTopTrader(entry.ProxyWallet, entry.PnL, estimatedWinRate); err != nil {
				log.Printf("Failed to upsert trader %s: %v", entry.ProxyWallet, err)
			} else {
				count++
				log.Printf("  ✓ Rank #%s: %s - PnL: $%.2f, Vol: $%.2f", 
					entry.Rank, entry.UserName, entry.PnL, entry.Vol)
			}
		} else {
			log.Printf("  ✗ Rank #%s: %s - PnL: $%.2f (below threshold)", 
				entry.Rank, entry.UserName, entry.PnL)
		}
	}

	log.Printf("✅ Updated leaderboard with %d profitable traders (out of %d total)", count, len(entries))
	
	// Log top traders we're tracking
	topTraders, err := i.db.GetTopTraders(i.cfg.TopTradersCount)
	if err == nil && len(topTraders) > 0 {
		log.Printf("📊 Currently tracking top %d traders:", len(topTraders))
		for idx, trader := range topTraders {
			log.Printf("   %d. %s", idx+1, trader)
		}
	}

	return nil
}

// GetLeaderboardWithParams allows custom API parameters
func (i *Ingestion) GetLeaderboardWithParams(ctx context.Context, timePeriod, orderBy string, limit int) ([]PolymarketLeaderboardEntry, error) {
	url := fmt.Sprintf("%s?timePeriod=%s&orderBy=%s&limit=%d&offset=0&category=overall",
		POLYMARKET_LEADERBOARD_API, timePeriod, orderBy, limit)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := i.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var entries []PolymarketLeaderboardEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}

	return entries, nil
}

// Mock function for testing
func (i *Ingestion) MockLeaderboard() error {
	mockTraders := []LeaderboardEntry{
		{Address: "0x9c667a1d1c1337c6dca9d93241d386e4ed346b66", PnL: 3868.57, WinRate: 0.65},
		{Address: "0xa61ef8773ec2e821962306ca87d4b57e39ff0abd", PnL: 3778.41, WinRate: 0.58},
	}

	for _, entry := range mockTraders {
		if err := i.db.UpsertTopTrader(entry.Address, entry.PnL, entry.WinRate); err != nil {
			return err
		}
	}

	log.Println("Loaded mock leaderboard data")
	return nil
}