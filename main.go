/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"os"
	rt "runtime"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	countermeasurev1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	eventsourcev1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/eventsource/v1alpha1"
	countermeasure "github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure"
	eventsource "github.com/dvilaverde/k8s-countermeasures/controllers/eventsource"
	"github.com/dvilaverde/k8s-countermeasures/pkg/dispatcher"
	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/dvilaverde/k8s-countermeasures/pkg/reconciler"
	"github.com/dvilaverde/k8s-countermeasures/pkg/sources"
	//+kubebuilder:scaffold:imports
)

const watchNamespaceEnvVar = "WATCH_NAMESPACE"

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(countermeasurev1alpha1.AddToScheme(scheme))
	utilruntime.Must(eventsourcev1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// TODO: re-enable
	// if !enableLeaderElection {
	// 	err := leader.Become(context.Background(), "countermeasure-op-lock")
	// 	if err != nil {
	// 		setupLog.Error(err, "unable to acquire leader lock")

	// 		os.Exit(21)
	// 	}
	// }

	watchNamespace, err := getWatchNamespace()
	if err != nil {
		setupLog.Error(err, `unable to get WatchNamespace, 
			the manager will watch and manage resources in all namespaces`)
	}

	managerOptions := ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "b444787d.vilaverde.rocks",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	}

	// if the watch namespaces are comma delimited, split and trim to create
	// a multi namespace cache.
	if strings.Contains(watchNamespace, ",") {
		managerOptions.Namespace = ""
		namespaces := strings.Split(watchNamespace, ",")
		for i := range namespaces {
			namespaces[i] = strings.TrimSpace(namespaces[i])
		}
		managerOptions.NewCache = cache.MultiNamespacedCacheBuilder(namespaces)
	} else {
		managerOptions.Namespace = watchNamespace
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), managerOptions)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	eventManager := &events.Manager{}

	// dispatch will be started by the controller runtime and will receive events from an
	// event source and dispatch them to the event manager which implements the listener
	// interface.
	dispatcher := dispatcher.NewDispatcher(eventManager, rt.NumCPU())
	mgr.Add(dispatcher)

	cmr := &countermeasure.CounterMeasureReconciler{
		ReconcilerBase: reconciler.NewFromManager(mgr, mgr.GetEventRecorderFor("countermeasure_controller")),
		EventManager:   eventManager,
		Log:            ctrl.Log.WithName("controllers").WithName("countermeasure"),
	}
	if err = (cmr).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create countermeasure controller")
		os.Exit(1)
	}

	sourceManager := &sources.Manager{
		Dispatcher: dispatcher,
	}
	// the source manager is a operator manager because it will be listening to the
	// done channel in order to stop any running event sources.
	mgr.Add(sourceManager)
	if err = (&eventsource.PrometheusReconciler{
		ReconcilerBase: reconciler.NewFromManager(mgr, mgr.GetEventRecorderFor("prometheus_controller")),
		SourceManager:  sourceManager,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Prometheus")
		os.Exit(1)
	}

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err = (&countermeasurev1alpha1.CounterMeasure{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "CounterMeasure")
			os.Exit(1)
		}

		if err = (&eventsourcev1alpha1.Prometheus{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Prometheus")
			os.Exit(1)
		}
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	setupLog.Info(fmt.Sprintf("monitoring namespaces: %s", watchNamespace))
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func getWatchNamespace() (string, error) {
	ns, found := os.LookupEnv(watchNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", watchNamespaceEnvVar)
	}
	return ns, nil
}
