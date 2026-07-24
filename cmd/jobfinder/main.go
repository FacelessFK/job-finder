// Command jobfinder سرویس را اجرا می‌کند: گرفتن، فیلتر و انتشار جاب‌های جدید.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aghaie/job-finder/internal/config"
	"github.com/aghaie/job-finder/internal/filter"
	"github.com/aghaie/job-finder/internal/pipeline"
	"github.com/aghaie/job-finder/internal/providers"
	"github.com/aghaie/job-finder/internal/publisher/telegram"
	"github.com/aghaie/job-finder/internal/store"

	// ثبت Providerها (افزودن منبع جدید = یک خط blank-import اینجا).
	_ "github.com/aghaie/job-finder/internal/providers/jsearch"
	_ "github.com/aghaie/job-finder/internal/providers/linkedin"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.Load(os.Getenv("CONFIG_PATH"))
	if err != nil {
		log.Error("config error", "err", err)
		os.Exit(1)
	}

	provs, err := providers.Build(cfg.Providers, cfg)
	if err != nil {
		log.Error("provider build error", "err", err)
		os.Exit(1)
	}

	st, err := store.NewFileStore(cfg.SeenPath, cfg.MaxSeen)
	if err != nil {
		log.Error("store error", "err", err)
		os.Exit(1)
	}

	pub, err := telegram.New(cfg)
	if err != nil {
		log.Error("publisher error", "err", err)
		os.Exit(1)
	}

	chain := filter.BuildRuleChain(log, cfg)
	p := pipeline.New(provs, chain, st, pub, log, cfg)

	if _, err := p.Run(context.Background()); err != nil {
		log.Error("run error", "err", err)
		os.Exit(1)
	}
}
