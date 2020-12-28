package eats_test

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/stset"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("InstanceIndexEnvInjector [needs-logs-for: eirini-api, instance-index-env-injector]", func() {
	var (
		namespace   string
		lrpGUID     string
		lrpVersion  string
		lrpName     string
		appListOpts metav1.ListOptions
	)

	getStatefulSetPods := func() []corev1.Pod {
		podList, err := fixture.Clientset.
			CoreV1().
			Pods(fixture.Namespace).
			List(context.Background(), appListOpts)

		Expect(err).NotTo(HaveOccurred())
		if len(podList.Items) == 0 {
			return nil
		}

		return podList.Items
	}

	getCFInstanceIndex := func(pod corev1.Pod) string {
		for _, container := range pod.Spec.Containers {
			if container.Name != stset.OPIContainerName {
				continue
			}

			for _, e := range container.Env {
				if e.Name != eirini.EnvCFInstanceIndex {
					continue
				}

				return e.Value
			}
		}

		return ""
	}

	BeforeEach(func() {
		namespace = fixture.Namespace
		lrpName = tests.GenerateGUID()
		lrpGUID = tests.GenerateGUID()
		lrpVersion = tests.GenerateGUID()
		appListOpts = metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s,%s=%s", stset.LabelGUID, lrpGUID, stset.LabelVersion, lrpVersion),
		}

		lrp := &eiriniv1.LRP{
			ObjectMeta: metav1.ObjectMeta{
				Name: lrpName,
			},
			Spec: eiriniv1.LRPSpec{
				GUID:                   lrpGUID,
				Version:                lrpVersion,
				Image:                  "eirini/dorini",
				AppGUID:                "the-app-guid",
				AppName:                "k-2so",
				SpaceName:              "s",
				OrgName:                "o",
				Env:                    map[string]string{"FOO": "BAR"},
				MemoryMB:               256,
				DiskMB:                 256,
				CPUWeight:              10,
				Instances:              3,
				LastUpdated:            "a long time ago in a galaxy far, far away",
				Ports:                  []int32{8080},
				VolumeMounts:           []eiriniv1.VolumeMount{},
				UserDefinedAnnotations: map[string]string{},
				AppRoutes:              []eiriniv1.Route{{Hostname: "app-hostname-1", Port: 8080}},
			},
		}

		_, err := fixture.EiriniClientset.
			EiriniV1().
			LRPs(namespace).
			Create(context.Background(), lrp, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		backgroundPropagation := metav1.DeletePropagationBackground

		err := fixture.EiriniClientset.
			EiriniV1().
			LRPs(fixture.Namespace).
			DeleteCollection(context.Background(),
				metav1.DeleteOptions{
					PropagationPolicy: &backgroundPropagation,
				},
				metav1.ListOptions{
					FieldSelector: "metadata.name=" + lrpName,
				},
			)
		Expect(err).NotTo(HaveOccurred())
	})

	It("creates pods with CF_INSTANCE_INDEX set to 0, 1 and 2", func() {
		Eventually(getStatefulSetPods, "30s").Should(HaveLen(3))

		envVars := []string{}
		for _, pod := range getStatefulSetPods() {
			envVars = append(envVars, getCFInstanceIndex(pod))
		}

		Expect(envVars).To(ConsistOf([]string{"0", "1", "2"}))
	})
})
