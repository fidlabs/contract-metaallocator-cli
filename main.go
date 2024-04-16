package main

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/build"
	lcli "github.com/filecoin-project/lotus/cli"
	cliutil "github.com/filecoin-project/lotus/cli/util"
	"github.com/filecoin-project/lotus/lib/lotuslog"
	"github.com/filecoin-project/lotus/lib/tracing"
	"github.com/filecoin-project/lotus/node/repo"
)

var log = logging.Logger("main")

func main() {
	lotuslog.SetupLogLevels()
	api.RunningNodeType = api.NodeFull

	jaeger := tracing.SetupJaegerTracing("contract-allocator-cli")
	defer func() {
		if jaeger != nil {
			_ = jaeger.ForceFlush(context.Background())
		}
	}()

	ctx, span := trace.StartSpan(context.Background(), "/cli")
	defer span.End()

	app := &cli.App{
		Name:                 "contract-allocator-cli",
		Usage:                "Filecoin Smart Contract Allocator client",
		Version:              build.UserVersion(),
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				// examined in the Before above
				Name:        "color",
				Usage:       "use color in display output",
				DefaultText: "depends on output being a TTY",
			},
			&cli.StringFlag{
				Name:    "repo",
				EnvVars: []string{"LOTUS_PATH"},
				Hidden:  true,
				Value:   "~/.lotus",
			},
			&cli.BoolFlag{
				Name:  "force-send",
				Usage: "if true, will ignore pre-send checks",
			},
			cliutil.FlagVeryVerbose,
		},

		Commands: []*cli.Command{
			FilplusDeployContractCmd,
			FilplusListContractsCmd,
			FilplusListAllocatorsCmd,
			FilplusAddAllowanceCmd,
			FilplusSetAllowanceCmd,
			FilplusAddVerifiedClientCmd,
			lcli.VersionCmd,
		},
	}

	app.Setup()
	app.Metadata["traceContext"] = ctx
	app.Metadata["repoType"] = repo.FullNode

	lcli.RunApp(app)
}
