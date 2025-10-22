// internal/database/database.go
package database

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

type User struct {
	ID            int64
	Address       string
	DepositAmount float64
	Shares        float64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Position struct {
	ID            int64
	MarketID      string
	TokenID       string
	Outcome       string
	Amount        float64
	AvgPrice      float64
	CurrentPrice  float64
	Status        string // "open", "closed"
	CreatedAt     time.Time
	ClosedAt      *time.Time
}

type Trade struct {
	ID            int64
	PositionID    int64
	TraderAddress string // Top trader we're copying
	Side          string // "buy", "sell"
	Amount        float64
	Price         float64
	TxHash        string
	Status        string // "pending", "confirmed", "failed"
	CreatedAt     time.Time
}

func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(); err != nil {
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		address TEXT UNIQUE NOT NULL,
		deposit_amount REAL NOT NULL DEFAULT 0,
		shares REAL NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS positions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		market_id TEXT NOT NULL,
		token_id TEXT NOT NULL,
		outcome TEXT NOT NULL,
		amount REAL NOT NULL,
		avg_price REAL NOT NULL,
		current_price REAL NOT NULL,
		status TEXT NOT NULL DEFAULT 'open',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		closed_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS trades (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		position_id INTEGER,
		trader_address TEXT NOT NULL,
		side TEXT NOT NULL,
		amount REAL NOT NULL,
		price REAL NOT NULL,
		tx_hash TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (position_id) REFERENCES positions(id)
	);

	CREATE TABLE IF NOT EXISTS top_traders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		address TEXT UNIQUE NOT NULL,
		total_pnl REAL NOT NULL,
		win_rate REAL NOT NULL,
		last_updated DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_positions_status ON positions(status);
	CREATE INDEX IF NOT EXISTS idx_trades_status ON trades(status);
	CREATE INDEX IF NOT EXISTS idx_users_address ON users(address);
	`

	_, err := db.conn.Exec(schema)
	return err
}

func (db *DB) Close() error {
	return db.conn.Close()
}

// User operations
func (db *DB) CreateUser(address string, depositAmount float64) (*User, error) {
	// Simple share calculation: 1:1 for now
	shares := depositAmount
	
	result, err := db.conn.Exec(
		"INSERT INTO users (address, deposit_amount, shares) VALUES (?, ?, ?)",
		address, depositAmount, shares,
	)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &User{
		ID:            id,
		Address:       address,
		DepositAmount: depositAmount,
		Shares:        shares,
		CreatedAt:     time.Now(),
	}, nil
}

func (db *DB) GetUser(address string) (*User, error) {
	user := &User{}
	err := db.conn.QueryRow(
		"SELECT id, address, deposit_amount, shares, created_at, updated_at FROM users WHERE address = ?",
		address,
	).Scan(&user.ID, &user.Address, &user.DepositAmount, &user.Shares, &user.CreatedAt, &user.UpdatedAt)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return user, err
}

// Position operations
func (db *DB) CreatePosition(marketID, tokenID, outcome string, amount, price float64) (*Position, error) {
	result, err := db.conn.Exec(
		"INSERT INTO positions (market_id, token_id, outcome, amount, avg_price, current_price) VALUES (?, ?, ?, ?, ?, ?)",
		marketID, tokenID, outcome, amount, price, price,
	)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &Position{
		ID:           id,
		MarketID:     marketID,
		TokenID:      tokenID,
		Outcome:      outcome,
		Amount:       amount,
		AvgPrice:     price,
		CurrentPrice: price,
		Status:       "open",
		CreatedAt:    time.Now(),
	}, nil
}

func (db *DB) GetOpenPositions() ([]Position, error) {
	rows, err := db.conn.Query(
		"SELECT id, market_id, token_id, outcome, amount, avg_price, current_price, status, created_at FROM positions WHERE status = 'open'",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []Position
	for rows.Next() {
		var p Position
		if err := rows.Scan(&p.ID, &p.MarketID, &p.TokenID, &p.Outcome, &p.Amount, &p.AvgPrice, &p.CurrentPrice, &p.Status, &p.CreatedAt); err != nil {
			return nil, err
		}
		positions = append(positions, p)
	}
	return positions, nil
}

// Trade operations
func (db *DB) CreateTrade(positionID int64, traderAddr, side string, amount, price float64) (*Trade, error) {
	result, err := db.conn.Exec(
		"INSERT INTO trades (position_id, trader_address, side, amount, price, status) VALUES (?, ?, ?, ?, ?, ?)",
		positionID, traderAddr, side, amount, price, "pending",
	)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &Trade{
		ID:            id,
		PositionID:    positionID,
		TraderAddress: traderAddr,
		Side:          side,
		Amount:        amount,
		Price:         price,
		Status:        "pending",
		CreatedAt:     time.Now(),
	}, nil
}

func (db *DB) UpdateTradeStatus(tradeID int64, status, txHash string) error {
	_, err := db.conn.Exec(
		"UPDATE trades SET status = ?, tx_hash = ? WHERE id = ?",
		status, txHash, tradeID,
	)
	return err
}

// Top traders
func (db *DB) UpsertTopTrader(address string, pnl, winRate float64) error {
	_, err := db.conn.Exec(`
		INSERT INTO top_traders (address, total_pnl, win_rate, last_updated)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(address) DO UPDATE SET
			total_pnl = excluded.total_pnl,
			win_rate = excluded.win_rate,
			last_updated = CURRENT_TIMESTAMP
	`, address, pnl, winRate)
	return err
}

func (db *DB) GetTopTraders(limit int) ([]string, error) {
	rows, err := db.conn.Query(
		"SELECT address FROM top_traders ORDER BY total_pnl DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traders []string
	for rows.Next() {
		var addr string
		if err := rows.Scan(&addr); err != nil {
			return nil, err
		}
		traders = append(traders, addr)
	}
	return traders, nil
}