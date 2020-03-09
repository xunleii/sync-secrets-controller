package main

import (
	"github.com/spf13/pflag"
	"k8s.io/component-base/logs"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/prometheus/common/version"
	kflag "k8s.io/component-base/cli/flag"

	"github.com/xunleii/sync-secrets-operator/pkg/controller"
)

var (
	ignoreNamespaces []string
)

func main() {
	pflag.StringSliceVar(&ignoreNamespaces, "ignore-namespace", nil, "List of namespaces to be ignored by the controller")

	logs.InitLogs()
	kflag.InitFlags()

	klog.V(1).Infof("sync-secrets-operator version: %s", version.Info())

	ctrl := controller.NewController(ignoreNamespaces)
	ctrl.Run(signals.SetupSignalHandler())
}
