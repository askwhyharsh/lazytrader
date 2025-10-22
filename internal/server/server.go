// internal/server/server.go
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	// "strconv"

	"github.com/gorilla/mux"
	"github.com/askwhyharsh/lazytrader/internal/config"
	"github.com/askwhyharsh/lazytrader/internal/database"
	// "github.com/askwhyharsh/lazytrader/internal/executor"
)

type Server struct {
	cfg  *config.Config
	db   *database.DB
	// exec *executor.Executor
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type DepositRequest struct {
	Address string  `json:"address"`
	Amount  float64 `json:"amount"`
}

type TradeRequestAPI struct {
	MarketID string  `json:"market_id"`
	TokenID  string  `json:"token_id"`
	Outcome  string  `json:"outcome"`
	Side     string  `json:"side"`
	Amount   float64 `json:"amount"`
	Price    float64 `json:"price"`
}

func New(cfg *config.Config, db *database.DB) *Server {
	return &Server{
		cfg:  cfg,
		db:   db,
		// exec: exec,
	}
}

func (s *Server) Start() error {
	r := mux.NewRouter()

	// API routes
	r.HandleFunc("/health", s.handleHealth).Methods("GET")
	// r.HandleFunc("/vault/info", s.handleVaultInfo).Methods("GET")
	// r.HandleFunc("/users", s.handleGetUsers).Methods("GET")
	// r.HandleFunc("/users/{address}", s.handleGetUser).Methods("GET")
	// r.HandleFunc("/deposit", s.handleDeposit).Methods("POST")
	// r.HandleFunc("/positions", s.handleGetPositions).Methods("GET")
	// r.HandleFunc("/trades/execute", s.handleExecuteTrade).Methods("POST")
	r.HandleFunc("/leaderboard", s.handleLeaderboard).Methods("GET")
	r.HandleFunc("/leaderboard/refresh", s.handleRefreshLeaderboard).Methods("POST")

	// addr := fmt.Sprintf(":%s", s.cfg.HTTPPort)
	log.Printf("Starting HTTP server on %s", ":4000")
	return http.ListenAndServe(":4000", r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, Response{Success: true, Data: "OK"})
}

// func (s *Server) handleVaultInfo(w http.ResponseWriter, r *http.Request) {
// 	balance, _ := s.exec.GetVaultBalance()
// 	shares, _ := s.exec.CalculateTotalShares()
// 	value, _ := s.exec.CalculateVaultValue()

// 	info := map[string]interface{}{
// 		"balance_wei": balance.String(),
// 		"total_shares": shares,
// 		"vault_value": value,
// 	}
// 	s.jsonResponse(w, Response{Success: true, Data: info})
// }

// func (s *Server) handleGetUsers(w http.ResponseWriter, r *http.Request) {
// 	// TODO: Implement get all users
// 	s.jsonResponse(w, Response{Success: true, Data: []interface{}{}})
// }

// func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
// 	vars := mux.Vars(r)
// 	address := vars["address"]

// 	user, err := s.db.GetUser(address)
// 	if err != nil {
// 		s.jsonError(w, fmt.Sprintf("Failed to get user: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	if user == nil {
// 		s.jsonError(w, "User not found", http.StatusNotFound)
// 		return
// 	}

// 	s.jsonResponse(w, Response{Success: true, Data: user})
// }

// func (s *Server) handleDeposit(w http.ResponseWriter, r *http.Request) {
// 	var req DepositRequest
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		s.jsonError(w, "Invalid request", http.StatusBadRequest)
// 		return
// 	}

// 	user, err := s.db.CreateUser(req.Address, req.Amount)
// 	if err != nil {
// 		s.jsonError(w, fmt.Sprintf("Failed to create user: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	s.jsonResponse(w, Response{Success: true, Data: user})
// }

// func (s *Server) handleGetPositions(w http.ResponseWriter, r *http.Request) {
// 	positions, err := s.db.GetOpenPositions()
// 	if err != nil {
// 		s.jsonError(w, fmt.Sprintf("Failed to get positions: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	s.jsonResponse(w, Response{Success: true, Data: positions})
// }

// func (s *Server) handleExecuteTrade(w http.ResponseWriter, r *http.Request) {
// 	var req TradeRequestAPI
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		s.jsonError(w, "Invalid request", http.StatusBadRequest)
// 		return
// 	}

// 	tradeReq := executor.TradeRequest{
// 		MarketID: req.MarketID,
// 		TokenID:  req.TokenID,
// 		Outcome:  req.Outcome,
// 		Side:     req.Side,
// 		Amount:   req.Amount,
// 		Price:    req.Price,
// 	}

// 	if err := s.exec.ExecuteTrade(tradeReq); err != nil {
// 		s.jsonError(w, fmt.Sprintf("Failed to execute trade: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	s.jsonResponse(w, Response{Success: true, Data: "Trade executed"})
// }

func (s *Server) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	traders, err := s.db.GetTopTraders(limit)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Failed to get leaderboard: %v", err), http.StatusInternalServerError)
		return
	}

	// Get full details for each trader
	var leaderboard []map[string]interface{}
	for _, trader := range traders {
		// TODO: Get full trader details from database
		leaderboard = append(leaderboard, map[string]interface{}{
			"address": trader,
		})
	}

	s.jsonResponse(w, Response{Success: true, Data: leaderboard})
}

func (s *Server) handleRefreshLeaderboard(w http.ResponseWriter, r *http.Request) {
	// Trigger an immediate leaderboard refresh
	// This would need to be implemented in the ingestion service
	s.jsonResponse(w, Response{Success: true, Data: "Leaderboard refresh triggered"})
}

func (s *Server) jsonResponse(w http.ResponseWriter, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) jsonError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(Response{Success: false, Error: message})
}