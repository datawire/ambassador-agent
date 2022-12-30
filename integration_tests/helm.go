package itest

import (
	"context"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
)

type InstallationConfig struct {
	ReleaseName string
	Namespace   string
	ChartDir    string
	Values      map[string]any

	RESTConfig *rest.Config
	Log        action.DebugLog
}

func InstallHelmChart(ctx context.Context, config InstallationConfig) (CleanupFunc, error) {
	var (
		helmCliConfig = genericclioptions.NewConfigFlags(false)
		actionConfig  action.Configuration

		restConf = config.RESTConfig
	)

	helmCliConfig.APIServer = &restConf.Host
	helmCliConfig.BearerToken = &restConf.BearerToken
	helmCliConfig.CAFile = &restConf.CAFile
	helmCliConfig.Namespace = &config.Namespace

	err := actionConfig.Init(helmCliConfig, config.Namespace, "", config.Log)
	if err != nil {
		return nil, err
	}

	chart, err := loader.LoadDir(config.ChartDir)
	if err != nil {
		return nil, err
	}

	install := action.NewInstall(&actionConfig)
	install.ReleaseName = config.ReleaseName
	install.Namespace = config.Namespace

	release, err := install.RunWithContext(ctx, chart, config.Values)
	if err != nil {
		return nil, err
	}

	return func(_ context.Context) error {
		_, err := action.NewUninstall(&actionConfig).Run(release.Name)
		return err
	}, nil
}
