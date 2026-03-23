package prerender

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"prerender-shield/internal/prerender/pool"
)

func BenchmarkPoolAcquireRelease(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	cfg := pool.DefaultConfig()
	cfg.MinInstances = 2
	cfg.MaxInstances = 5

	p := pool.NewPool(cfg, logger)
	defer p.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		instance, err := p.Acquire(ctx)
		if err != nil {
			b.Fatal(err)
		}

		_ = instance.ID

		err = p.Release(instance)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPoolAcquireReleaseParallel(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	cfg := pool.DefaultConfig()
	cfg.MinInstances = 3
	cfg.MaxInstances = 10

	p := pool.NewPool(cfg, logger)
	defer p.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			instance, err := p.Acquire(ctx)
			if err != nil {
				b.Fatal(err)
			}

			_ = instance.ID

			err = p.Release(instance)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
