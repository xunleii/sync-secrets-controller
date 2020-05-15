package main

import (
	"github.com/spf13/pflag"
	"k8s.io/component-base/logs"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/prometheus/common/version"
	kflag "k8s.io/component-base/cli/flag"

	"github.com/xunleii/sync-secrets-controller/pkg/controller"
)

const (
	controllerName       = "sync-secrets-controller"
	controllerNameMetric = "sync_secrets_controller"
)

func main() {
	var ctx controller.Context
	var metricsBindAddress, healthProbeBindAddress string

	pflag.StringVar(&metricsBindAddress, "metrics-bind-address", ":8080", "Address to bind to access to the metrics")
	pflag.StringVar(&healthProbeBindAddress, "health-probe-bind-address", ":8081", "Address to bind to access to health probes")
	pflag.StringSliceVar(&ctx.IgnoredNamespaces, "ignore-namespaces", []string{"kube-system"}, "List of namespaces to be ignored by the controller")

	pflag.StringSliceVar(&ctx.ProtectedLabels, "protected-labels", nil, "List of protected labels which must not be copied")
	pflag.StringSliceVar(&ctx.ProtectedAnnotations, "protected-annotations", nil, "List of protected annotations which must not be copied")

	logs.InitLogs()
	kflag.InitFlags()

	klog.V(1).Infof("%s version: %s", controllerName, version.Info())
	klog.V(4).Infof(version.Print(controllerName))
	metrics.Registry.MustRegister(version.NewCollector(controllerNameMetric))

	ctrl := controller.NewController(metricsBindAddress, healthProbeBindAddress, ctx)
	ctrl.Run(signals.SetupSignalHandler())
}
