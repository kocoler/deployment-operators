/*


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
	appsv1alpha1 "github.com/kocoler/deployment-operators/api/v1alpha1"
	"github.com/kocoler/deployment-operators/controllers"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"net/http"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sync"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
	entryLog = ctrl.Log.WithName("entrypoint")
	oriLog   = ctrl.Log
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = appsv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":9090", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "7024fd2d.my.domain",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.DeploymentOperatorsReconciler{
		Client:      mgr.GetClient(),
		Log:         oriLog.WithName("controllers").WithName("deploymentOperators"),
		Scheme:      mgr.GetScheme(),
		Deployments: sync.Map{},
		Replicas:    sync.Map{},
		PendingTask: sync.Map{},
		Count:       0,
		MessageSenderInstance: &controllers.MessageSender{
			Hosts:          nil,
			MessagesBuffer: []controllers.Message{},
			ClientPool: sync.Pool{
				New: func() interface{} {
					return new(http.Client)
				},
			},
			Log: oriLog,
		},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DeploymentOperators")
		os.Exit(1)
	}

	//// Setup a new controller to reconcile ReplicaSets
	//entryLog.Info("Setting up controller")
	//c, err := controller.New("deployment-operator-controller", mgr, controller.Options{
	//	Reconciler: &controllers.DeploymentOperatorsReconciler{Client: mgr.GetClient()},
	//})
	//if err != nil {
	//	entryLog.Error(err, "unable to set up individual controller")
	//	os.Exit(1)
	//}
	//
	//// Watch Deployment and enqueue Deployment object key
	//if err := c.Watch(&source.Kind{Type: &appsv1.Deployment{}},
	//	&handler.EnqueueRequestForObject{}); err != nil {
	//	entryLog.Error(err, "unable to watch deployments")
	//	os.Exit(1)
	//}
	//
	//// Watch Pods and enqueue owning ReplicaSet key
	//if err := c.Watch(&source.Kind{Type: &appsv1.ReplicaSet{}},
	//	&handler.EnqueueRequestForOwner{OwnerType: &appsv1.ReplicaSet{}, IsController: true}); err != nil {
	//	entryLog.Error(err, "unable to watch Pods")
	//	os.Exit(1)
	//}

	// Setup webhooks
	entryLog.Info("setting up webhook server")
	//hookServer := mgr.GetWebhookServer()

	entryLog.Info("registering webhooks to the webhook server")
	//hookServer.Register("/validate-v1-pod", &webhook.Admission{Handler: &podwebhook.PodCollector{Client: mgr.GetClient()}})

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
