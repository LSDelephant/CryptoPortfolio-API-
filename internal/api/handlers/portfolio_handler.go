package handlers

import (
	"crypto-portfolio-api/internal/models"
	"crypto-portfolio-api/internal/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PortfolioHandler struct {
	portfolioService *services.PortfolioService
	priceService     *services.PriceService
}

func NewPortfolioHandler(portfolioService *services.PortfolioService, priceService *services.PriceService) *PortfolioHandler {
	return &PortfolioHandler{
		portfolioService: portfolioService,
		priceService:     priceService,
	}
}

// @Summary Create new portfolio
// @Description Create a new portfolio for authenticated user
// @Tags portfolios
// @Accept json
// @Produce json
// @Param portfolio body models.CreatePortfolioRequest true "Portfolio data"
// @Success 201 {object} models.Portfolio
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/portfolios [post]
func (h *PortfolioHandler) CreatePortfolio(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req models.CreatePortfolioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	portfolio, err := h.portfolioService.CreatePortfolio(c.Request.Context(), userID.(uuid.UUID), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create portfolio"})
		return
	}

	c.JSON(http.StatusCreated, portfolio)
}

// @Summary Get user portfolios
// @Description Get all portfolios for authenticated user
// @Tags portfolios
// @Produce json
// @Success 200 {array} models.Portfolio
// @Failure 401 {object} map[string]string
// @Router /api/portfolios [get]
func (h *PortfolioHandler) GetUserPortfolios(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	portfolios, err := h.portfolioService.GetUserPortfolios(c.Request.Context(), userID.(uuid.UUID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error
