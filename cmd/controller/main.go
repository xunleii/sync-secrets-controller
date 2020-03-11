package main

import (
	"github.com/spf13/pflag"
	"k8s.io/component-base/logs"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/prometheus/common/version"
	kflag "k8s.io/component-base/cli/flag"

	"github.com/xunleii/sync-secrets-operator/pkg/controller"
)

const (
	controllerName = "controller"
	controllerNameMetric = "sync_secrets_operator"
)

func main() {
	var ignoreNamespaces []string
	var metricsBindAddress, healthProbeBindAddress string

	pflag.StringVar(&metricsBindAddress, "metrics-bind-address", ":8080", "Address to bind to access to the metrics")
	pflag.StringVar(&healthProbeBindAddress, "health-probe-bind-address", ":8081", "Address to bind to access to health probes")
	pflag.StringSliceVar(&ignoreNamespaces, "ignore-namespaces", nil, "List of namespaces to be ignored by the controller")

	logs.InitLogs()
	kflag.InitFlags()

	klog.V(1).Infof("%s version: %s", controllerName, version.Info())
	klog.V(4).Infof(version.Print(controllerName))
	metrics.Registry.MustRegister(version.NewCollector(controllerNameMetric))

	ctrl := controller.NewController(metricsBindAddress, healthProbeBindAddress, ignoreNamespaces)
	ctrl.Run(signals.SetupSignalHandler())
}
