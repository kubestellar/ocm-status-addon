package main

import (
	"context"
	goflag "flag"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/rest"
	utilflag "k8s.io/component-base/cli/flag"
	featuregate "k8s.io/component-base/featuregate"
	logs "k8s.io/component-base/logs/api/v1"
	_ "k8s.io/component-base/logs/json/register"
	"k8s.io/klog/v2"

	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	addonagent "open-cluster-management.io/addon-framework/pkg/agent"
	cmdfactory "open-cluster-management.io/addon-framework/pkg/cmd/factory"
	"open-cluster-management.io/addon-framework/pkg/utils"
	"open-cluster-management.io/addon-framework/pkg/version"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonv1alpha1client "open-cluster-management.io/api/client/addon/clientset/versioned"
	clusterv1 "open-cluster-management.io/api/cluster/v1"

	"github.com/kubestellar/ocm-status-addon/pkg/agent"
	"github.com/kubestellar/ocm-status-addon/pkg/controller"
	"github.com/kubestellar/ocm-status-addon/pkg/observability"
)

func main() {
	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	features := featuregate.NewFeatureGate()
	err := features.Add(map[featuregate.Feature]featuregate.FeatureSpec{
		logs.ContextualLogging: {Default: true, PreRelease: featuregate.Alpha},

		logs.LoggingAlphaOptions: {Default: false, PreRelease: featuregate.Alpha},
		logs.LoggingBetaOptions:  {Default: true, PreRelease: featuregate.Beta},
	})
	if err != nil {
		panic(err)
	}
	logConfig := logs.NewLoggingConfiguration()
	logs.AddFlags(logConfig, pflag.CommandLine)

	command := newCommand(logConfig, features)
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func newCommand(logConfig *logs.LoggingConfiguration, features featuregate.FeatureGate) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addon",
		Short: "status addon",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return logs.ValidateAndApply(logConfig, features)
		},
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmd.Help(); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}
			os.Exit(1)
		},
	}

	if v := version.Get().String(); len(v) == 0 {
		cmd.Version = "<unknown>"
	} else {
		cmd.Version = v
	}

	cmd.AddCommand(newControllerCommand())
	cmd.AddCommand(agent.NewAgentCommand("status"))

	return cmd
}

type agentController struct {
	ObservabilityOptions observability.ObservabilityOptions[*pflag.FlagSet]
	NameToWrapped        map[string]*pflag.Flag
}

func newControllerCommand() *cobra.Command {
	agentObservability := agent.NewObservabilityOptions()
	ac := agentController{
		ObservabilityOptions: observability.ObservabilityOptions[*pflag.FlagSet]{
			MetricsBindAddr: ":9280",
			PprofBindAddr:   ":9282",
		},
		NameToWrapped: make(map[string]*pflag.Flag)}
	agentLogConfig := logs.NewLoggingConfiguration()
	agentUserOptions := agent.NewAgentUserOptions()
	flagsOnAgent := pflag.NewFlagSet("on-agent", pflag.ContinueOnError)
	flagsFromAgent := pflag.NewFlagSet("from-agent", pflag.ContinueOnError)
	agentObservability.AddToFlagSet(flagsOnAgent)
	logs.AddFlags(agentLogConfig, flagsOnAgent)
	agentUserOptions.AddToFlagSet(flagsFromAgent)
	cmd := cmdfactory.
		NewControllerCommandConfig("status-addon-controller", version.Get(), ac.runController).
		NewCommand()
	cmd.Use = "controller"
	cmd.Short = "Start the addon controller"
	ac.ObservabilityOptions.AddToFlagSet(cmd.PersistentFlags())
	for connector, flagSet := range map[string]*pflag.FlagSet{"on": flagsOnAgent, "from": flagsFromAgent} {
		flagSet.VisitAll(func(flag *pflag.Flag) {
			wrapped := *flag
			wrapped.Name = "agent-" + flag.Name
			wrapped.Usage = flag.Usage + " " + connector + " the agent"
			wrapped.Shorthand = ""
			cmd.PersistentFlags().AddFlag(&wrapped)
			ac.NameToWrapped[flag.Name] = &wrapped
		})
	}
	return cmd
}

func (ac *agentController) getPropagatedSettings(*clusterv1.ManagedCluster, *addonapiv1alpha1.ManagedClusterAddOn) (addonfactory.Values, error) {
	settings := []string{}
	for flagName, wrapped := range ac.NameToWrapped {
		if wrapped.Changed {
			setting := "--" + flagName + "=" + wrapped.Value.String()
			settings = append(settings, setting)
		}
	}
	sort.Strings(settings)
	return map[string]any{"PropagatedSettings": settings}, nil
}

func (ac *agentController) runController(ctx context.Context, kubeConfig *rest.Config) error {
	ac.ObservabilityOptions.StartServing(ctx)
	addonClient, err := addonv1alpha1client.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}

	mgr, err := addonmanager.New(kubeConfig)
	if err != nil {
		return err
	}

	registrationOption := controller.NewRegistrationOption(
		kubeConfig,
		controller.AddonName,
		utilrand.String(5),
	)

	// Set agent install namespace from addon deployment config if it exists
	registrationOption.AgentInstallNamespace = utils.AgentInstallNamespaceFromDeploymentConfigFunc(
		utils.NewAddOnDeploymentConfigGetter(addonClient),
	)

	agentAddon, err := addonfactory.NewAgentAddonFactory(controller.AddonName, controller.FS, "manifests/templates").
		WithConfigGVRs(utils.AddOnDeploymentConfigGVR).
		WithGetValuesFuncs(
			controller.GetDefaultValues,
			ac.getPropagatedSettings,
			addonfactory.GetAddOnDeploymentConfigValues(
				utils.NewAddOnDeploymentConfigGetter(addonClient),
				addonfactory.ToAddOnDeploymentConfigValues,
				addonfactory.ToImageOverrideValuesFunc("Image", controller.DefaultStatusAddOnImage),
			),
		).
		WithAgentRegistrationOption(registrationOption).
		WithInstallStrategy(addonagent.InstallAllStrategy(controller.InstallationNamespace)).
		WithAgentHealthProber(controller.AgentHealthProber()).
		BuildTemplateAgentAddon()
	if err != nil {
		klog.Errorf("failed to build agent %v", err)
		return err
	}

	err = mgr.AddAgent(agentAddon)
	if err != nil {
		klog.Fatal(err)
	}

	err = mgr.Start(ctx)
	if err != nil {
		klog.Fatal(err)
	}
	<-ctx.Done()

	return nil
}
