package services

import (
	"context"
	"crypto-portfolio-api/internal/models"
	"crypto-portfolio-api/pkg/utils"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/go-redis/redis/v8"
)

type PortfolioService struct {
	db    *sql.DB
	redis *redis.Client
}

func NewPortfolioService(db *sql.DB, redis *redis.Client) *PortfolioService {
	return &PortfolioService{
		db:    db,
		redis: redis,
	}
}

func (s *PortfolioService) CreatePortfolio(ctx context.Context, userID uuid.UUID, req models.CreatePortfolioRequest) (*models.Portfolio, error) {
	portfolio := &models.Portfolio{
		ID:          uuid.New(),
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	query := `
		INSERT INTO portfolios (id, user_id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := s.db.ExecContext(ctx, query,
		portfolio.ID, portfolio.UserID, portfolio.Name,
		portfolio.Description, portfolio.CreatedAt, portfolio.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create portfolio: %w", err)
	}

	// Очищуємо кеш користувача
	s.clearUserPortfoliosCache(userID)

	return portfolio, nil
}

func (s *PortfolioService) GetUserPortfolios(ctx context.Context, userID uuid.UUID) ([]models.Portfolio, error) {
	cacheKey := fmt.Sprintf("user_portfolios:%s", userID.String())
	
	// Спочатку перевіряємо кеш
	cached, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var portfolios []models.Portfolio
		if err := json.Unmarshal([]byte(cached), &portfolios); err == nil {
			return portfolios, nil
		}
	}

	// Якщо в кеші немає, запитуємо з БД
	query := `
		SELECT p.id, p.user_id, p.name, p.description, p.created_at, p.updated_at
		FROM portfolios p
		WHERE p.user_id = $1
		ORDER BY p.created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get portfolios: %w", err)
	}
	defer rows.Close()

	var portfolios []models.Portfolio
	for rows.Next() {
		var p models.Portfolio
		err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan portfolio: %w", err)
		}
		portfolios = append(portfolios, p)
	}

	// Кешуємо результат на 5 хвилин
	if len(portfolios) > 0 {
		data, _ := json.Marshal(portfolios)
		s.redis.Set(ctx, cacheKey, data, 5*time.Minute)
	}

	return portfolios, nil
}

func (s *PortfolioService) GetPortfolioWithHoldings(ctx context.Context, portfolioID, userID uuid.UUID) (*models.Portfolio, error) {
	cacheKey := fmt.Sprintf("portfolio_full:%s", portfolioID.String())
	
	// Перевіряємо кеш
	cached, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var portfolio models.Portfolio
		if err := json.Unmarshal([]byte(cached), &portfolio); err == nil {
			return &portfolio, nil
		}
	}

	// Отримуємо портфоліо
	portfolio := &models.Portfolio{}
	query := `
		SELECT id, user_id, name, description, created_at, updated_at
		FROM portfolios
		WHERE id = $1 AND user_id = $2
	`
	
	err = s.db.QueryRowContext(ctx, query, portfolioID, userID).Scan(
		&portfolio.ID, &portfolio.UserID, &portfolio.Name,
		&portfolio.Description, &portfolio.CreatedAt, &portfolio.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get portfolio: %w", err)
	}

	// Отримуємо холдинги
	holdingsQuery := `
		SELECT id, portfolio_id, symbol, amount, buy_price, buy_date, created_at, updated_at
		FROM holdings
		WHERE portfolio_id = $1
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, holdingsQuery, portfolioID)
	if err != nil {
		return nil, fmt.Errorf("failed to get holdings: %w", err)
	}
	defer rows.Close()

	var holdings []models.Holding
	for rows.Next() {
		var h models.Holding
		err := rows.Scan(&h.ID, &h.PortfolioID, &h.Symbol, &h.Amount,
			&h.BuyPrice, &h.BuyDate, &h.CreatedAt, &h.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan holding: %w", err)
		}
		holdings = append(holdings, h)
	}

	portfolio.Holdings = holdings

	// Кешуємо на 2 хвилини
	data, _ := json.Marshal(portfolio)
	s.redis.Set(ctx, cacheKey, data, 2*time.Minute)

	return portfolio, nil
}

func (s *PortfolioService) AddHolding(ctx context.Context, portfolioID uuid.UUID, req models.AddHoldingRequest) (*models.Holding, error) {
	holding := &models.Holding{
		ID:          uuid.New(),
		PortfolioID: portfolioID,
		Symbol:      req.Symbol,
		Amount:      req.Amount,
		BuyPrice:    req.BuyPrice,
		BuyDate:     time.Now(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	query := `
		INSERT INTO holdings (id, portfolio_id, symbol, amount, buy_price, buy_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := s.db.ExecContext(ctx, query,
		holding.ID, holding.PortfolioID, holding.Symbol, holding.Amount,
		holding.BuyPrice, holding.BuyDate, holding.CreatedAt, holding.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add holding: %w", err)
	}

	// Очищуємо кеш портфоліо
	s.clearPortfolioCache(portfolioID)

	return holding, nil
}

func (s *PortfolioService) CalculatePortfolioStats(ctx context.Context, portfolioID uuid.UUID) (*models.PortfolioStats, error) {
	cacheKey := fmt.Sprintf("portfolio_stats:%s", portfolioID.String())
	
	// Перевіряємо кеш
	cached, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var stats models.PortfolioStats
		if err := json.Unmarshal([]byte(cached), &stats); err == nil {
			return &stats, nil
		}
	}

	// Отримуємо холдинги
	portfolio, err := s.GetPortfolioWithHoldings(ctx, portfolioID, uuid.Nil) // TODO: pass real user ID
	if err != nil {
		return nil, err
	}

	stats := &models.PortfolioStats{}
	var totalValue, totalCost float64
	bestPnL, worstPnL := -999999.0, 999999.0
	var bestPerformer, worstPerformer string

	for _, holding := range portfolio.Holdings {
		// Отримуємо поточну ціну з Redis
		priceKey := fmt.Sprintf("price:%s", holding.Symbol)
		priceStr, err := s.redis.Get(ctx, priceKey).Result()
		if err != nil {
			continue // Skip if price not available
		}

		var priceData models.PriceData
		if err := json.Unmarshal([]byte(priceStr), &priceData); err != nil {
			continue
		}

		currentValue := holding.Amount * priceData.Price
		cost := holding.Amount * holding.BuyPrice
		pnlPerc := ((priceData.Price - holding.BuyPrice) / holding.BuyPrice) * 100

		totalValue += currentValue
		totalCost += cost

		if pnlPerc > bestPnL {
			bestPnL = pnlPerc
			bestPerformer = holding.Symbol
		}
		if pnlPerc < worstPnL {
			worstPnL = pnlPerc
			worstPerformer = holding.Symbol
		}
	}

	stats.TotalValue = totalValue
	stats.TotalCost = totalCost
	stats.TotalPnL = totalValue - totalCost
	if totalCost > 0 {
		stats.TotalPnLPerc = (stats.TotalPnL / totalCost) * 100
	}
	stats.BestPerformer = bestPerformer
	stats.WorstPerformer = worstPerformer

	// Кешуємо на 1 хвилину
	data, _ := json.Marshal(stats)
	s.redis.Set(ctx, cacheKey, data, 1*time.Minute)

	return stats, nil
}

func (s *PortfolioService) clearUserPortfoliosCache(userID uuid.UUID) {
	cacheKey := fmt.Sprintf("user_portfolios:%s", userID.String())
	s.redis.Del(context.Background(), cacheKey)
}

func (s *PortfolioService) clearPortfolioCache(portfolioID uuid.UUID) {
	cacheKey := fmt.Sprintf("portfolio_full:%s", portfolioID.String())
	statsKey := fmt.Sprintf("portfolio_stats:%s", portfolioID.String())
	s.redis.Del(context.Background(), cacheKey, statsKey)
}
