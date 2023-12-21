package main

import (
	"context"
	goflag "flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/rest"
	utilflag "k8s.io/component-base/cli/flag"
	logs "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2"
	addonv1alpha1client "open-cluster-management.io/api/client/addon/clientset/versioned"

	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	addonagent "open-cluster-management.io/addon-framework/pkg/agent"
	cmdfactory "open-cluster-management.io/addon-framework/pkg/cmd/factory"
	"open-cluster-management.io/addon-framework/pkg/utils"
	"open-cluster-management.io/addon-framework/pkg/version"

	"github.ibm.com/dettori/status-addon/pkg/agent"
	"github.ibm.com/dettori/status-addon/pkg/controller"
)

func main() {
	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	logs.AddFlags(logs.NewLoggingConfiguration(), pflag.CommandLine)

	command := newCommand()
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func newCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addon",
		Short: "status addon",
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

func newControllerCommand() *cobra.Command {
	cmd := cmdfactory.
		NewControllerCommandConfig("status-addon-controller", version.Get(), runController).
		NewCommand()
	cmd.Use = "controller"
	cmd.Short = "Start the addon controller"

	return cmd
}

func runController(ctx context.Context, kubeConfig *rest.Config) error {
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
