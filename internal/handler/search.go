package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/TheTuxis/gondor-search/internal/model"
	"github.com/TheTuxis/gondor-search/internal/service"
)

type SearchHandler struct {
	searchService *service.SearchService
}

func NewSearchHandler(searchService *service.SearchService) *SearchHandler {
	return &SearchHandler{searchService: searchService}
}

// Search handles POST /v1/search
func (h *SearchHandler) Search(c *gin.Context) {
	var req model.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "invalid search request",
			"details": gin.H{"error": err.Error()},
		})
		return
	}

	// Use company_id from JWT if not explicitly set
	if req.CompanyID == 0 {
		if companyID, exists := c.Get("company_id"); exists {
			req.CompanyID = companyID.(uint)
		}
	}

	result, err := h.searchService.Search(c.Request.Context(), req)
	if err != nil {
		if err == service.ErrEmptyQuery || err == service.ErrInvalidEntity {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "validation_error",
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "search failed",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Suggest handles POST /v1/search/suggest
func (h *SearchHandler) Suggest(c *gin.Context) {
	var req model.SuggestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "invalid suggest request",
			"details": gin.H{"error": err.Error()},
		})
		return
	}

	if req.CompanyID == 0 {
		if companyID, exists := c.Get("company_id"); exists {
			req.CompanyID = companyID.(uint)
		}
	}

	result, err := h.searchService.Suggest(c.Request.Context(), req)
	if err != nil {
		if err == service.ErrEmptyQuery {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "validation_error",
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "suggest failed",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// IndexDocument handles POST /v1/search/index
func (h *SearchHandler) IndexDocument(c *gin.Context) {
	var doc model.IndexDocument
	if err := c.ShouldBindJSON(&doc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "invalid index document",
			"details": gin.H{"error": err.Error()},
		})
		return
	}

	if err := h.searchService.IndexDocument(c.Request.Context(), doc); err != nil {
		if err == service.ErrInvalidEntity {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "validation_error",
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "failed to index document",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "document indexed",
		"id":      doc.ID,
	})
}

// DeleteDocument handles DELETE /v1/search/index/:entity_type/:id
func (h *SearchHandler) DeleteDocument(c *gin.Context) {
	entityType := c.Param("entity_type")
	id := c.Param("id")

	if err := h.searchService.DeleteDocument(c.Request.Context(), entityType, id); err != nil {
		if err == service.ErrInvalidEntity {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "validation_error",
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "failed to delete document",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "document deleted",
	})
}

// Reindex handles POST /v1/search/reindex/:entity_type
func (h *SearchHandler) Reindex(c *gin.Context) {
	entityType := c.Param("entity_type")

	if err := h.searchService.Reindex(c.Request.Context(), entityType); err != nil {
		if err == service.ErrInvalidEntity {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "validation_error",
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "reindex failed",
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":     "reindex triggered",
		"entity_type": entityType,
	})
}
