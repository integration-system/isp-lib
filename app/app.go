package app

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/integration-system/isp-lib/v3/config"
	"github.com/integration-system/isp-lib/v3/log"
	"github.com/integration-system/isp-lib/v3/validator"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type Runner interface {
	Run(ctx context.Context) error
}

type Closer interface {
	Close() error
}

type Application struct {
	ctx    context.Context
	cfg    *config.Config
	logger *log.Adapter

	group   *errgroup.Group
	runners []Runner
	closers []Closer
}

func New() *Application {
	isDev := strings.ToLower(os.Getenv("APP_MODE")) == "dev"
	group, ctx := errgroup.WithContext(context.Background())

	localConfigOpts := []config.LocalOption{
		config.WithValidator(validator.Default),
		config.WithEnvPrefix("LC_ISP"),
	}
	cfgFile, err := configFile(isDev)
	if err != nil {
		fmt.Println(errors.Wrap(err, "resolve config file path"))
		os.Exit(1)
		return nil
	}
	localConfigOpts = append(localConfigOpts, config.WithReadingFromFile(cfgFile))

	cfg, err := config.New(localConfigOpts...)
	if err != nil {
		fmt.Println(errors.Wrap(err, "create config"))
		os.Exit(1)
		return nil
	}

	loggerOpts := []log.Option{log.WithDevelopmentMode(), log.WithLevel(log.DebugLevel)}
	if !isDev {
		loggerOpts = []log.Option{log.WithLevel(log.InfoLevel)}
		logFilePath := cfg.GetString("LOG_FILE_PATH")
		if logFilePath != "" {
			rotation := log.Rotation{
				File:       logFilePath,
				MaxSizeMb:  512,
				MaxDays:    0,
				MaxBackups: 4,
				Compress:   true,
			}
			loggerOpts = append(loggerOpts, log.WithFileRotation(rotation))
		}
	}
	logger, err := log.New(loggerOpts...)
	if err != nil {
		fmt.Println(errors.Wrap(err, "create logger"))
		os.Exit(1)
		return nil
	}

	return &Application{
		ctx:     ctx,
		cfg:     cfg,
		logger:  logger,
		group:   group,
		closers: []Closer{logger},
	}
}

func (a Application) Context() context.Context {
	return a.ctx
}

func (a Application) Config() *config.Config {
	return a.cfg
}

func (a Application) Logger() log.Logger {
	return a.logger
}

func (a *Application) AddRunners(runners ...Runner) {
	a.runners = append(a.runners, runners...)
}

func (a *Application) AddClosers(closers ...Closer) {
	a.closers = append(a.closers, closers...)
}

func (a *Application) Run() error {
	for i := range a.runners {
		runner := a.runners[i]
		a.group.Go(func() error {
			err := runner.Run(a.ctx)
			if err != nil {
				return errors.Wrapf(err, "start runner[%s]", runner)
			}
			return nil
		})
	}
	return a.group.Wait()
}

func (a *Application) Shutdown() {
	for i := len(a.closers); i > 0; i-- {
		closer := a.closers[i-1]
		err := closer.Close()
		if err != nil {
			a.logger.Error(a.ctx, err, log.String("closer", fmt.Sprintln(closer)))
		}
	}
}

func configFile(isDev bool) (string, error) {
	cfgPath := os.Getenv("APP_CONFIG_PATH")
	if cfgPath != "" {
		return cfgPath, nil
	}
	if isDev {
		return "./conf/config.yml", nil
	}
	ex, err := os.Executable()
	if err != nil {
		return "", errors.Wrap(err, "get executable path")
	}
	return path.Join(path.Dir(ex), "config.yml"), nil
}
