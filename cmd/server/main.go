package main

import (
	"context"
	"crypto-portfolio-api/internal/config"
	"crypto-portfolio-api/internal/api/routes"
	"crypto-portfolio-api/pkg/database"
	"crypto-portfolio-api/pkg/redis"
	"crypto-portfolio-api/internal/services"
	"crypto-portfolio-api/internal/websocket"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Завантажуємо конфігурацію
	cfg := config.Load()

	// Підключаємо базу даних
	db, err := database.NewConnection(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Підключаємо Redis
	redisClient, err := redis.NewClient(cfg.RedisURL)
	if err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}
	defer redisClient.Close()

	// Ініціалізуємо сервіси
	portfolioService := services.NewPortfolioService(db, redisClient)
	priceService := services.NewPriceService(redisClient)
	authService := services.NewAuthService(db, cfg.JWTSecret)

	// Запускаємо WebSocket хаб
	hub := websocket.NewHub()
	go hub.Run()

	// Запускаємо оновлення цін у фоні
	go priceService.StartPriceUpdater(hub)

	// Налаштовуємо Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	// Налаштовуємо маршрути
	routes.SetupRoutes(router, portfolioService, priceService, authService, hub)

	// Додаємо метрики Prometheus
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Запускаємо сервер
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Server starting on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed to start:", err)
		}
	}()

	// Чекаємо сигнал завершення
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}
