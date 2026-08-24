package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuery_ResgisterExecution(t *testing.T) {
	casos := []struct {
		nome              string
		execCountInicial  int64
		meanTimeMsInicial float64
		m2Inicial         float64
		execMs            float64
		esperaAnomalia    bool
		esperaZScore      float64
	}{
		{nome: "antes do cold start",
			execCountInicial:  3,
			meanTimeMsInicial: 50,
			m2Inicial:         100,
			execMs:            9999,
			esperaAnomalia:    false,
			esperaZScore:      0,
		},
		{nome: "execução normal",
			execCountInicial:  8,
			meanTimeMsInicial: 50,
			m2Inicial:         175,
			execMs:            52,
			esperaAnomalia:    false,
			esperaZScore:      0,
		},
		{nome: "estoura o z-score",
			execCountInicial:  8,
			meanTimeMsInicial: 50,
			m2Inicial:         175,
			execMs:            90,
			esperaAnomalia:    true,
			esperaZScore:      8.0,
		},
		{nome: "desvio zero, execução igual",
			execCountInicial:  8,
			meanTimeMsInicial: 50,
			m2Inicial:         0,
			execMs:            50,
			esperaAnomalia:    false,
			esperaZScore:      0,
		},
		{nome: "desvio zero, execução diferente",
			execCountInicial:  8,
			meanTimeMsInicial: 50,
			m2Inicial:         0,
			execMs:            999,
			esperaAnomalia:    true,
			esperaZScore:      0,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			query := Query{
				ExecutionsCount: caso.execCountInicial,
				MeanTimeMs:      caso.meanTimeMsInicial,
				M2:              caso.m2Inicial,
			}

			result := query.RegisterExecution(caso.execMs)

			assert.Equal(t, caso.esperaAnomalia, result.IsAnomaly)
			assert.InDelta(t, caso.esperaZScore, result.ZScore, 0.01)
		})
	}
}
