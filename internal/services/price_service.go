package services

import (
	"context"
	"crypto-portfolio-api/internal/models"
	"crypto-portfolio-api/internal/websocket"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

type PriceService struct {
	redis      *redis.Client
	httpClient *http.Client
}

type CoinGeckoResponse struct {
	Data map[string]CoinGeckoPrice `json:""`
}

type CoinGeckoPrice struct {
	USD                   float64 `json:"usd"`
	USD24hChange          float64 `json:"usd_24h_change"`
	USD24hVol             float64 `json:"usd_24h_vol"`
	USDMarketCap          float64 `json:"usd_market_cap"`
	LastUpdatedAt         int64   `json:"last_updated_at"`
}

func NewPriceService(redis *redis.Client) *PriceService {
	return &PriceService{
		redis: redis,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *PriceService) StartPriceUpdater(hub *websocket.Hub) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Список популярних криптовалют
	symbols := []string{"bitcoin", "ethereum", "binancecoin", "cardano", "solana", "polkadot"}

	for {
		select {
		case <-ticker.C:
			if err := s.updatePrices(symbols, hub); err != nil {
				log.Printf("Error updating prices: %v", err)
			}
		}
	}
}

func (s *PriceService) updatePrices(symbols []string, hub *websocket.Hub) error {
	ctx := context.Background()
	
	// Формуємо запит до CoinGecko API
	symbolsStr := strings.Join(symbols, ",")
	url := fmt.Sprintf(
		"https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd&include_24hr_change=true&include_24hr_vol=true&include_market_cap=true&include_last_updated_at=true",
		symbolsStr,
	)

	resp, err := s.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch prices: %w", err)
	}
	defer resp.Body.Close()

	var coinGeckoData map[string]CoinGeckoPrice
	if err := json.NewDecoder(resp.Body).Decode(&coinGeckoData); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Обробляємо та кешуємо дані
	var priceUpdates []models.PriceData
	for coinId, priceInfo := range coinGeckoData {
		// Конвертуємо ID в символ (спрощений варіант)
		symbol := strings.ToUpper(coinId)
		if coinId == "binancecoin" {
			symbol = "BNB"
		}

		priceData := models.PriceData{
			Symbol:      symbol,
			Price:       priceInfo.USD,
			Change24h:   priceInfo.USD24hChange,
			Volume24h:   priceInfo.USD24hVol,
			MarketCap:   priceInfo.USDMarketCap,
			LastUpdated: time.Unix(priceInfo.LastUpdatedAt, 0),
		}

		// Зберігаємо в Redis
		data, _ := json.Marshal(priceData)
		cacheKey := fmt.Sprintf("price:%s", symbol)
		s.redis.Set(ctx, cacheKey, data, 5*time.Minute)

		priceUpdates = append(priceUpdates, priceData)
	}

	// Відправляємо оновлення через WebSocket
	if hub != nil {
		update := map[string]interface{}{
			"type":   "price_update",
			"prices": priceUpdates,
		}
		hub.Broadcast <- update
	}

	return nil
}

func (s *PriceService) GetPrice(ctx context.Context, symbol string) (*models.PriceData, error) {
	cacheKey := fmt.Sprintf("price:%s", strings.ToUpper(symbol))
	
	cached, err := s.redis.Get(ctx, cacheKey).Result()
	if err != nil {
		return nil, fmt.Errorf("price not found for symbol %s", symbol)
	}

	var priceData models.PriceData
	if err := json.Unmarshal([]byte(cached), &priceData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal price data: %w", err)
	}

	return &priceData, nil
}

func (s *PriceService) GetMultiplePrices(ctx context.Context, symbols []string) (map[string]models.PriceData, error) {
	result := make(map[string]models.PriceData)
	
	for _, symbol := range symbols {
		if priceData, err := s.GetPrice(ctx, symbol); err == nil {
			result[symbol] = *priceData
		}
	}
	
	return result, nil
}
