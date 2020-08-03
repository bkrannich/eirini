package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/k8s/reconciler"
	eirinischeme "code.cloudfoundry.org/eirini/pkg/generated/clientset/versioned/scheme"
	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	ctrl "sigs.k8s.io/controller-runtime"

	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running eirini-controller"`
}

func main() {
	if err := kscheme.AddToScheme(eirinischeme.Scheme); err != nil {
		cmdcommons.Exitf("failed to add the k8s scheme to the LRP CRD scheme: %v", err)
	}

	var opts options
	_, err := flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitIfError(err)

	eiriniCfg, err := readConfigFile(opts.ConfigFile)
	cmdcommons.ExitIfError(err)

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", eiriniCfg.Properties.ConfigPath)
	cmdcommons.ExitIfError(err)

	controllerClient, err := runtimeclient.New(kubeConfig, runtimeclient.Options{Scheme: eirinischeme.Scheme})
	cmdcommons.ExitIfError(err)

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	cmdcommons.ExitIfError(err)

	logger := lager.NewLogger("eirini-informer")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{
		Scheme: eirinischeme.Scheme,
	})
	cmdcommons.ExitIfError(err)
	lrpReconciler := createLRPReconciler(logger.Session("lrp-reconciler"), controllerClient, clientset, eiriniCfg, mgr.GetScheme())
	taskReconciler := createTaskReconciler(logger.Session("task-reconciler"), controllerClient, clientset, eiriniCfg, mgr.GetScheme())

	err = builder.
		ControllerManagedBy(mgr).
		For(&eiriniv1.LRP{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(lrpReconciler)
	cmdcommons.ExitIfError(err)

	err = builder.
		ControllerManagedBy(mgr).
		For(&eiriniv1.Task{}).
		Owns(&batchv1.Job{}).
		Complete(taskReconciler)
	cmdcommons.ExitIfError(err)

	predicates := []predicate.Predicate{labelPredicate{}}
	err = builder.
		ControllerManagedBy(mgr).
		For(&corev1.Pod{}, builder.WithPredicates(predicates...)).
		Complete(podCrashReconciler{client: controllerClient})
	cmdcommons.ExitIfError(err)

	err = mgr.Start(ctrl.SetupSignalHandler())
	cmdcommons.ExitIfError(err)
}

type podCrashReconciler struct {
	client runtimeclient.Client
}

func (r podCrashReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	pod := &corev1.Pod{}
	r.client.Get(context.TODO(), request.NamespacedName, pod)
	fmt.Printf("Pod Reconciler: %s / %s - %+v\n", request.Namespace, request.Name, pod)
	return reconcile.Result{}, nil
}

type labelPredicate struct{}

// Create returns true if the Create event should be processed
func (p labelPredicate) Create(event.CreateEvent) bool {
	return false
}

func (p labelPredicate) Delete(event.DeleteEvent) bool {
	return false
}
func (p labelPredicate) Update(e event.UpdateEvent) bool {
	labels := e.MetaNew.GetLabels()
	sourceType := labels[k8s.LabelSourceType]
	fmt.Printf("sourceType was %q\n", sourceType)
	return sourceType == "APP"
}
func (p labelPredicate) Generic(event.GenericEvent) bool {
	return false
}

func readConfigFile(path string) (*eirini.Config, error) {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	var conf eirini.Config
	err = yaml.Unmarshal(fileBytes, &conf)
	return &conf, errors.Wrap(err, "failed to unmarshal yaml")
}

func createLRPReconciler(
	logger lager.Logger,
	controllerClient runtimeclient.Client,
	clientset kubernetes.Interface,
	eiriniCfg *eirini.Config,
	scheme *runtime.Scheme) *reconciler.LRP {
	stDesirer := &k8s.StatefulSetDesirer{
		Pods:                              client.NewPod(clientset),
		Secrets:                           client.NewSecret(clientset),
		StatefulSets:                      client.NewStatefulSet(clientset),
		PodDisruptionBudets:               client.NewPodDisruptionBudget(clientset),
		Events:                            client.NewEvent(clientset),
		StatefulSetToLRPMapper:            k8s.StatefulSetToLRP,
		RegistrySecretName:                eiriniCfg.Properties.RegistrySecretName,
		RootfsVersion:                     eiriniCfg.Properties.RootfsVersion,
		LivenessProbeCreator:              k8s.CreateLivenessProbe,
		ReadinessProbeCreator:             k8s.CreateReadinessProbe,
		Logger:                            logger,
		ApplicationServiceAccount:         eiriniCfg.Properties.ApplicationServiceAccount,
		AllowAutomountServiceAccountToken: eiriniCfg.Properties.UnsafeAllowAutomountServiceAccountToken,
	}

	return reconciler.NewLRP(logger, controllerClient, stDesirer, client.NewStatefulSet(clientset), scheme)
}

func createTaskReconciler(
	logger lager.Logger,
	controllerClient runtimeclient.Client,
	clientset kubernetes.Interface,
	eiriniCfg *eirini.Config,
	scheme *runtime.Scheme) *reconciler.Task {
	taskDesirer := k8s.NewTaskDesirer(
		logger,
		client.NewJob(clientset),
		client.NewSecret(clientset),
		"",
		[]k8s.StagingConfigTLS{},
		eiriniCfg.Properties.ApplicationServiceAccount,
		"",
		eiriniCfg.Properties.RegistrySecretName,
		eiriniCfg.Properties.RootfsVersion,
	)

	return reconciler.NewTask(logger, controllerClient, taskDesirer, scheme)
}
