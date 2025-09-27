package models

import (
	"time"
	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	Password  string    `json:"-" db:"password_hash"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type Portfolio struct {
	ID          uuid.UUID `json:"id" db:"id"`
	UserID      uuid.UUID `json:"user_id" db:"user_id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
	Holdings    []Holding `json:"holdings,omitempty"`
}

type Holding struct {
	ID          uuid.UUID `json:"id" db:"id"`
	PortfolioID uuid.UUID `json:"portfolio_id" db:"portfolio_id"`
	Symbol      string    `json:"symbol" db:"symbol"`
	Amount      float64   `json:"amount" db:"amount"`
	BuyPrice    float64   `json:"buy_price" db:"buy_price"`
	BuyDate     time.Time `json:"buy_date" db:"buy_date"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type PriceData struct {
	Symbol      string    `json:"symbol"`
	Price       float64   `json:"price"`
	Change24h   float64   `json:"change_24h"`
	Volume24h   float64   `json:"volume_24h"`
	MarketCap   float64   `json:"market_cap"`
	LastUpdated time.Time `json:"last_updated"`
}

type PortfolioStats struct {
	TotalValue    float64 `json:"total_value"`
	TotalCost     float64 `json:"total_cost"`
	TotalPnL      float64 `json:"total_pnl"`
	TotalPnLPerc  float64 `json:"total_pnl_percentage"`
	BestPerformer string  `json:"best_performer"`
	WorstPerformer string `json:"worst_performer"`
}

// Запити API
type CreatePortfolioRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=100"`
	Description string `json:"description" binding:"max=500"`
}

type AddHoldingRequest struct {
	Symbol   string  `json:"symbol" binding:"required,uppercase,min=1,max=10"`
	Amount   float64 `json:"amount" binding:"required,gt=0"`
	BuyPrice float64 `json:"buy_price" binding:"required,gt=0"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}
