package lens_test

import (
	"context"
	"os"
	"testing"

	"github.com/biggz-ai/biggz/internal/lens/dependencies"
	"github.com/biggz-ai/biggz/internal/lens/performance"
	"github.com/biggz-ai/biggz/internal/lens/readability"
	"github.com/biggz-ai/biggz/internal/lens/reliability"
	"github.com/biggz-ai/biggz/internal/lens/resilience"
	"github.com/biggz-ai/biggz/internal/lens/risk"
	"github.com/biggz-ai/biggz/model"
	"github.com/biggz-ai/biggz/plugin"
)

// BenchmarkLensRisk benchmarks the Risk lens analysis.
func BenchmarkLensRisk(b *testing.B) {
	lens := &risk.RiskLens{}
	subject := largeSubject()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := lens.Analyze(context.Background(), subject)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLensReadability benchmarks the Readability lens.
func BenchmarkLensReadability(b *testing.B) {
	lens := &readability.ReadabilityLens{}
	subject := largeSubject()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := lens.Analyze(context.Background(), subject)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLensReliability benchmarks the Reliability lens.
func BenchmarkLensReliability(b *testing.B) {
	lens := &reliability.ReliabilityLens{}
	subject := largeSubject()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := lens.Analyze(context.Background(), subject)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLensResilience benchmarks the Resilience lens.
func BenchmarkLensResilience(b *testing.B) {
	lens := &resilience.ResilienceLens{}
	subject := largeSubject()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := lens.Analyze(context.Background(), subject)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLensPerformance benchmarks the Performance lens.
func BenchmarkLensPerformance(b *testing.B) {
	lens := &performance.PerformanceLens{}
	subject := largeSubject()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := lens.Analyze(context.Background(), subject)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLensDependencies benchmarks the Dependencies lens.
func BenchmarkLensDependencies(b *testing.B) {
	lens := &dependencies.DependenciesLens{}
	subject := largeSubject()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := lens.Analyze(context.Background(), subject)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAllLenses benchmarks all 6 lenses in sequence (simulating DAG).
func BenchmarkAllLenses(b *testing.B) {
	lenses := []plugin.LensPlugin{
		&risk.RiskLens{},
		&readability.ReadabilityLens{},
		&reliability.ReliabilityLens{},
		&resilience.ResilienceLens{},
		&performance.PerformanceLens{},
		&dependencies.DependenciesLens{},
	}
	subject := largeSubject()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, lens := range lenses {
			_, err := lens.Analyze(context.Background(), subject)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

func largeSubject() model.ReviewSubject {
	cwd, _ := os.Getwd()
	return model.ReviewSubject{
		Repository: cwd,
		// Use HEAD as the commit — lenses will run git diff against the
		// current working tree, measuring actual analysis cost.
		CommitSHA: "HEAD",
	}
}

// BenchmarkResult is a no-op to prevent compiler optimization.
var BenchmarkResult interface{}

func BenchmarkFullPipeline(b *testing.B) {
	lenses := []plugin.LensPlugin{
		&risk.RiskLens{},
		&readability.ReadabilityLens{},
		&reliability.ReliabilityLens{},
		&resilience.ResilienceLens{},
		&performance.PerformanceLens{},
		&dependencies.DependenciesLens{},
	}
	subject := largeSubject()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var results []*plugin.LensResult
		for _, l := range lenses {
			r, err := l.Analyze(context.Background(), subject)
			if err != nil {
				b.Fatal(err)
			}
			results = append(results, r)
		}
		BenchmarkResult = results
	}
}
