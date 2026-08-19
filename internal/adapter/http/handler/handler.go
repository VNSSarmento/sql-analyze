package handler

import (
	"errors"
	"net/http"
	"sql-analyze/internal/domain"
	"sql-analyze/internal/usecase"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service         *usecase.AnalyzeQueryUseCase
	listUseCase     *usecase.ListSlowestQueriesUseCase
	getQueryUseCase *usecase.GetQueryUseCase
}

func NewHandler(analyzeUsecase *usecase.AnalyzeQueryUseCase, listSlowestQueriesUseCase *usecase.ListSlowestQueriesUseCase, getQueryUserCase *usecase.GetQueryUseCase) *Handler {
	return &Handler{
		service:         analyzeUsecase,
		listUseCase:     listSlowestQueriesUseCase,
		getQueryUseCase: getQueryUserCase,
	}
}

func (h *Handler) GetSlowestQueries(ctx *gin.Context) {
	var finalLimit int
	defaultLimit := 10
	maxLimit := 100

	limitString := ctx.Query("limit")

	if limitString == "" {
		finalLimit = defaultLimit
	} else {
		limitInt, err := strconv.Atoi(limitString)

		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "O parâmetro limit deve ser um número válido"})
			return
		}

		if limitInt <= 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "O parâmetro limit deve ser maior que zero"})
			return
		}

		if limitInt > maxLimit {
			finalLimit = maxLimit
		} else {
			finalLimit = limitInt
		}
	}

	domainQueries, err := h.listUseCase.Execute(ctx, finalLimit)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Erro interno"})
		return
	}

	responseJson := mapToSlowQueryResponseList(domainQueries)

	ctx.JSON(http.StatusOK, responseJson)
}

func (h *Handler) GetQueryById(ctx *gin.Context) {
	queryID := ctx.Param("queryID")
	dbUser := ctx.Param("dbUser")

	query, err := h.getQueryUseCase.Execute(ctx, queryID, dbUser)

	if errors.Is(err, domain.ErrQueryNotFound) {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Não foi possivel localizar a query"})
		return
	}

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Erro interno"})
		return
	}

	responseJson := mapToSlowQueryResponse(query)

	ctx.JSON(http.StatusOK, responseJson)

}
