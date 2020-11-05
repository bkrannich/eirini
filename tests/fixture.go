package tests

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"

	eiriniclient "code.cloudfoundry.org/eirini/pkg/generated/clientset/versioned"
	"github.com/hashicorp/go-multierror"

	// nolint:golint,stylecheck
	. "github.com/onsi/ginkgo"

	// nolint:golint,stylecheck
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	basePortNumber = 20000
	portRange      = 1000
)

type Fixture struct {
	Clientset         kubernetes.Interface
	EiriniClientset   eiriniclient.Interface
	Namespace         string
	PspName           string
	KubeConfigPath    string
	Writer            io.Writer
	nextAvailablePort int
	portMux           *sync.Mutex
	extraNamespaces   []string
}

func makeKubeConfigCopy() string {
	kubeConfig := GetKubeconfig()
	if kubeConfig == "" {
		return ""
	}

	tmpKubeConfig, err := ioutil.TempFile("", "kube.cfg")
	Expect(err).NotTo(HaveOccurred())

	defer tmpKubeConfig.Close()

	kubeConfigContents, err := os.Open(kubeConfig)
	Expect(err).NotTo(HaveOccurred())

	defer kubeConfigContents.Close()

	_, err = io.Copy(tmpKubeConfig, kubeConfigContents)
	Expect(err).NotTo(HaveOccurred())

	return tmpKubeConfig.Name()
}

func NewFixture(writer io.Writer) *Fixture {
	kubeConfigPath := makeKubeConfigCopy()
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	Expect(err).NotTo(HaveOccurred(), "failed to build config from flags")

	clientset, err := kubernetes.NewForConfig(config)
	Expect(err).NotTo(HaveOccurred(), "failed to create clientset")

	lrpclientset, err := eiriniclient.NewForConfig(config)
	Expect(err).NotTo(HaveOccurred(), "failed to create clientset")

	return &Fixture{
		KubeConfigPath:    kubeConfigPath,
		Clientset:         clientset,
		EiriniClientset:   lrpclientset,
		Writer:            writer,
		nextAvailablePort: basePortNumber + portRange*GinkgoParallelNode(),
		portMux:           &sync.Mutex{},
	}
}

func (f *Fixture) SetUp() {
	f.Namespace = f.CreateExtraNamespace()
}

func (f *Fixture) NextAvailablePort() int {
	f.portMux.Lock()
	defer f.portMux.Unlock()

	if f.nextAvailablePort > f.maxPortNumber() {
		Fail("Ginkgo node %d is not allowed to allocate more than %d ports", GinkgoParallelNode(), portRange)
	}

	port := f.nextAvailablePort
	f.nextAvailablePort++

	return port
}

func (f Fixture) maxPortNumber() int {
	return basePortNumber + portRange*GinkgoParallelNode() + portRange
}

func (f *Fixture) TearDown() {
	f.printDebugInfo()

	for _, ns := range f.extraNamespaces {
		_ = f.deleteNamespace(ns)
	}
}

func (f *Fixture) Destroy() {
	Expect(os.RemoveAll(f.KubeConfigPath)).To(Succeed())
}

func (f *Fixture) CreateExtraNamespace() string {
	name := f.configureNewNamespace()
	f.extraNamespaces = append(f.extraNamespaces, name)

	return name
}

func (f *Fixture) configureNewNamespace() string {
	namespace := CreateRandomNamespace(f.Clientset)
	Expect(CreatePodCreationPSP(namespace, getPspName(namespace), GetApplicationServiceAccount(), f.Clientset)).To(Succeed(), "failed to create pod creation PSP")

	namespacedRole := &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eirini-namespaced-role",
			Namespace: namespace,
		},
		Rules: []v1.PolicyRule{
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs"},
				Verbs:     []string{"create", "delete", "update", "patch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"statefulsets"},
				Verbs:     []string{"create", "update", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"delete", "patch"},
			},
			{
				APIGroups: []string{"policy"},
				Resources: []string{"poddisruptionbudgets"},
				Verbs:     []string{"create", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"create", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "update", "delete"},
			},
			{
				APIGroups: []string{"eirini.cloudfoundry.org"},
				Resources: []string{"lrps/status"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"eirini.cloudfoundry.org"},
				Resources: []string{"lrps/status"},
				Verbs:     []string{"update"},
			},
		},
	}
	_, err := f.Clientset.RbacV1().Roles(namespace).Create(context.Background(), namespacedRole, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	namespacedRoleBinding := &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eirini-namespaced-rolebinding",
			Namespace: namespace,
		},
		Subjects: []v1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "opi",
				Namespace: "eirini-core",
			},
			{
				Kind:      "ServiceAccount",
				Name:      "eirini-controller",
				Namespace: "eirini-core",
			},
			{
				Kind:      "ServiceAccount",
				Name:      "eirini-task-reporter",
				Namespace: "eirini-core",
			},
		},
		RoleRef: v1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "eirini-namespaced-role",
		},
	}
	_, err = f.Clientset.RbacV1().RoleBindings(namespace).Create(context.Background(), namespacedRoleBinding, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	return namespace
}

func (f *Fixture) deleteNamespace(namespace string) error {
	var errs *multierror.Error
	errs = multierror.Append(errs, DeleteNamespace(namespace, f.Clientset))
	errs = multierror.Append(errs, DeletePSP(getPspName(namespace), f.Clientset))

	return errs.ErrorOrNil()
}

//nolint:gocyclo
func (f *Fixture) printDebugInfo() {
	fmt.Fprintln(f.Writer, "Jobs:")

	jobs, _ := f.Clientset.BatchV1().Jobs(f.Namespace).List(context.Background(), metav1.ListOptions{})

	for _, job := range jobs.Items {
		fmt.Fprintf(f.Writer, "Job: %s status is: %#v\n", job.Name, job.Status)
		fmt.Fprintln(f.Writer, "-----------")
	}

	statefulsets, _ := f.Clientset.AppsV1().StatefulSets(f.Namespace).List(context.Background(), metav1.ListOptions{})

	fmt.Fprintf(f.Writer, "StatefulSets:")

	for _, s := range statefulsets.Items {
		fmt.Fprintf(f.Writer, "StatefulSet: %s status is: %#v\n", s.Name, s.Status)
		fmt.Fprintln(f.Writer, "-----------")
	}

	pods, _ := f.Clientset.CoreV1().Pods(f.Namespace).List(context.Background(), metav1.ListOptions{})

	fmt.Fprintf(f.Writer, "Pods:")

	for _, p := range pods.Items {
		fmt.Fprintf(f.Writer, "Pod: %s status is: %#v\n", p.Name, p.Status)
		fmt.Fprintln(f.Writer, "-----------")

		fmt.Fprintf(f.Writer, "Pod: %s logs are: \n", p.Name)
		logsReq := f.Clientset.CoreV1().Pods(f.Namespace).GetLogs(p.Name, &corev1.PodLogOptions{})

		if err := consumeRequest(logsReq, f.Writer); err != nil {
			fmt.Fprintf(f.Writer, "Failed to get logs for Pod: %s becase: %v \n", p.Name, err)
		}
	}
}

func consumeRequest(request rest.ResponseWrapper, out io.Writer) error {
	readCloser, err := request.Stream(context.Background())
	if err != nil {
		return err
	}
	defer readCloser.Close()

	r := bufio.NewReader(readCloser)

	for {
		bytes, err := r.ReadBytes('\n')
		if _, writeErr := out.Write(bytes); writeErr != nil {
			return writeErr
		}

		if err != nil {
			if !errors.Is(err, io.EOF) {
				return err
			}

			return nil
		}
	}
}

func getPspName(namespace string) string {
	return fmt.Sprintf("%s-psp", namespace)
}
